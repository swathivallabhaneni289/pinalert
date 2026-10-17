# Phase 8: Report page and comments - Context

**Gathered:** 2026-10-17 and 2026-10-18, from the owner's answers in the discuss round run by session pinalert-60, on top of the design the owner LOCKED on 2026-10-17 from the clickable mockup `.planning/sketches/008-report-page/` (ROADMAP Phase 8 scope notes 5 and 6).
**Status:** Ready for research and planning. The page design is locked: changes go back to the owner first. The new sign in (scope note 6) is shown to the owner as a mockup before it is built. Everything tagged CLAUDE'S DISCRETION stands unless the owner vetoes it at the viewing session.

<domain>
## Phase Boundary

A reader opens any report into its own page (beside the map on a laptop, full screen on a phone) with the popup's trust block, a history timeline built from the vote log, comments with replies and thumbs up likes, and Share. A shared link opens a small public status page for that one report, readable without an account. Everyone gets a public username. Sign up and sign in move from the emailed link to a password with one typed 6-digit email code, and step 4 of the first-time flow becomes required.

Not in this phase: profiles, follows and stars (Phase 9), the official alerts and the Demo (Phase 5), phone notifications (Phase 3.3).

Requirements: PAGE-01, PAGE-02, COORD-08, and the IDENT-01 change (sign in by password with a typed code).
</domain>

<decisions>
## Implementation Decisions

### How to read the status tags
- **OWNER DECIDED**: the owner said it in this round. Do not re-ask.
- **LOCKED EARLIER**: decided in the ROADMAP, the locked sketch 008 or an earlier phase. Do not re-ask.
- **CLAUDE'S DISCRETION**: the owner was not asked. Build the default named here unless the owner vetoes it at the viewing session.

### The report page (locked)
- **D-01 [LOCKED EARLIER, sketch 008 and ROADMAP scope note 5]:** Laptop: the report opens beside the map in place of the Nearby list (B), every pin still on the map, another pin switches the report. Phone: a full screen page whose map keeps every pin. Tapping a pin opens its report straight away on both sizes. The page uses the popup's trust block as shipped (sketch 003 H), not a table. Order: the report, the trust block, actions (Confirm, Dispute, or Mark resolved with Undo on your own; Share; quiet links Stop updates and Flag), "What happened so far" timeline from the vote log with no voter names or places, then Comments. Keep it calm ("make sure it doesnt look too busy"). Every detail is in `.planning/sketches/008-report-page/README.md`.
- **D-02 [LOCKED EARLIER, sketch 008 item 5]:** Comments: the same list on phone and laptop under a "Comments" heading with the count, Instagram like; each comment has a round picture, the username, the time, the words, Reply, then a thumbs up like with its count (one per account per comment, never on the report, never feeding any trust number, never a heart). Replies folded behind "View N replies", one level only. One box: Reply fills "@username ", shows "Replying to username" with an X. 280 letters, "N left" only in the last 40. The box is pinned to the bottom of the panel on a laptop and sits after the last comment on a phone. Your own comments show only your username, never "You", and no Reporter tag on your own report; others see Reporter beside the reporter's name.
- **D-03 [LOCKED EARLIER, ROADMAP SC4 and scope note 3]:** The popup footer shows a real comment count only, never a made up one.

### Sharing (COORD-08)
- **D-04 [LOCKED EARLIER, sketch 008 item 6]:** Share opens a window (a bottom sheet on the phone) showing what people get in WhatsApp: the title and "Open to see if this is still true right now", with a link and no counts, so a forward can never carry a stale number or a "verified" claim.
- **D-05 [OWNER DECIDED, 2026-10-17]:** The shared link opens a **small public page for that one report, with no sign in**. It shows only: what it is and the rough area; whether it is still happening, confirmed, disputed or over; how many people nearby confirmed it; and when. It never shows comments, names or the exact spot. A "Sign in to see more" button sits at the bottom. Chosen over requiring an account.
- **D-06 [CLAUDE'S DISCRETION]:** Public page details: "what it is" is the category, the severity and, for a Critical report, the What is happening pick; the reporter's own words are not shown, matching the lock screen rule (03.3 D-14). The rough area is a coarse area name, and finished Rescue reports keep their 1 km coarsening (03.1 D-23). A report removed by a moderator shows "This report was removed". The page is `noindex` and `no-store`, rate limited per IP, and its link preview (Open Graph) carries only the title line and "Open to see if this is still true right now", never a count. The route joins the gate's exempt list in `internal/api/gate_test.go` with a note.

### Accounts, names and sign up
- **D-07 [OWNER DECIDED, 2026-10-17]:** An account is needed to use Pinalert ("should have an account no matter what"). Only these open without one: the Demo (Phase 5), the Disclaimer and Policy pages, and the D-05 shared report page.
- **D-08 [OWNER DECIDED, 2026-10-17, chosen over the advice to make Name optional]:** Step 4 of the first-time flow (About you) requires all three: **Username** (live check), **Name** and **Home area** (a town or area, never a street). No Skip. The owner: "do have the name because ... for the profile page thing ... you know how the Instagram works it should work exactly like that". Today's step 4 says "Both are optional. They show only if you later let others follow you." and has Skip (03.4 as built); this phase changes it once. Earlier accounts fill in whatever is missing once, at their next sign in (scope note 5).
- **D-09 [LOCKED EARLIER, scope note 5]:** The Username is unique and public to everyone, Instagram style; the Name stays a profile only detail (Phase 9). The Username is editable in Settings with a live check (available, taken, wrong characters, or kept for official use such as police, ndma, imd, govt) and is screened like comments.
- **D-10 [LOCKED EARLIER, scope note 6]:** Sign in with a password. The sign in page offers Create account and Sign in. Create account takes an email and a password, emails a 6-digit code once, the person types it in (no link anywhere), then the first-time flow runs. Sign in takes the email and the password only. A forgotten password is reset with the same kind of typed code. Accounts made with the old link set a password once at their next sign in, inside D-08's one-time fill-in, so nobody is stopped twice. Engineering notes from the scope note stand (slow hash, per account and per device limits mindful of CGNAT, "Email or password is wrong", same "we sent a code" screen for a known email, short lived single use hashed codes).
- **D-11 [CLAUDE'S DISCRETION]:** Planning may split the new sign in (D-10) and the required step 4 (D-08) into their own decimal phase before the report page, because they replace the whole sign in and need their own mockup. If split, the two ship together so step 4 changes only once. The Phase 5 globe and the "Try the demo · Disclaimer · Policy" row must survive the new card (05-CONTEXT D-03, D-14).

### Comments: rules the owner was not asked about
- **D-12 [CLAUDE'S DISCRETION]:** Screening follows Phase 4's report rule: clearly abusive text is rejected with the box kept open; borderline text posts and is listed for moderators; if the check cannot finish, the comment is held off the page until it does (comments have no critical exemption). One reusable text check with a held state, not tied to report categories, serves comments and usernames (ROADMAP scope note 4). Comment text never reaches the page through a markup parsing sink.
- **D-13 [CLAUDE'S DISCRETION]:** A per account limit on comments, replies and likes, with a message that says how long to wait, built on Phase 4's limiter; numbers set at planning.
- **D-14 [CLAUDE'S DISCRETION]:** You can delete your own comment at any time; there is no editing (as on Instagram). A deleted comment that has replies leaves "Comment deleted" so the thread still reads. The reporter cannot delete other people's comments on their report; anyone signed in can Flag a comment with Phase 4's quiet control, and moderators Keep or Remove it on the moderation screen. A flag alone never hides a comment.
- **D-15 [CLAUDE'S DISCRETION]:** Comments stay open while the report is current and become read only once it is over (resolved or expired), with a quiet "Comments are closed" line.
- **D-16 [CLAUDE'S DISCRETION]:** A reply to you and a new comment on your own report appear in your in-app updates (Phase 3.2 bell). Likes never notify. Phone notifications for these use the existing "changes to reports I posted or voted on" kind (off by default) and, like every phone notification, are silent (owner, 2026-10-18: no buzzes anywhere).
- **D-17 [CLAUDE'S DISCRETION]:** Ship order inside the phase: the page, the timeline and Share first; then comments (they need the moderation hook and limiter).

### Claude's Discretion (summary)
D-06, D-11 to D-17.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### The locked design
- `.planning/sketches/008-report-page/README.md`: every page, comment and Share detail, and the owner decisions of 2026-10-17. The clickable mockup is `.planning/sketches/008-report-page/index.html`.
- `.planning/sketches/003-trust-block-redesign/`: the trust block (variant H) the page reuses.
- `.planning/sketches/MANIFEST.md`: row 008 still says the username is asked at the first comment; reconcile it with D-08 at planning.

### Scope and earlier decisions
- `.planning/ROADMAP.md` Phase 8 section: goal, SC1 to SC5, scope notes 1 to 6.
- `.planning/REQUIREMENTS.md`: PAGE-01, PAGE-02, COORD-08, IDENT-01 (updated at planning).
- `.planning/PROJECT.md`: Access model (updated at planning for passwords).
- `.planning/phases/01.1-identity-login-mandatory-email-verification/01.1-CONTEXT.md`: D-05 (login required), the link flow being replaced.
- `.planning/phases/03.4-app-layout/03.4-CONTEXT.md`: D-24b, D-25 (Settings), D-27 (first-time flow step 4 as built).
- `.planning/phases/03.1-report-history-views/03.1-CONTEXT.md`: D-11 (`/?report=ID` link), D-23 (finished Rescue coarsening).
- `.planning/phases/03.2-in-app-updates-and-trusted-reporter-alerts/03.2-CONTEXT.md`: the updates pipeline comment events plug into.
- `.planning/phases/04-robustness-real-world-resilience/`: the text screening, the flag control, the moderation screen and the limiter.
- `.planning/sketches/009-login-globe/README.md`: "More, from reading the code", the sign in rough edges, most of which go away with the link flow.
- `.planning/phases/05-official-feed-demo-mode/05-CONTEXT.md`: D-03, D-04, D-14 (Demo links and the globe behind the sign in card).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/templates/welcome.html.tmpl` step 4 (About you: Name and Home area, Continue and Skip); `handlers.UpdateProfile` (`PATCH /api/me/profile`) and migration `00008_add_onboarding_and_happening.sql` (`display_name`, `home_area_name`, `home_area_cell`).
- `web/templates/login_gate.html.tmpl`, `check_inbox.html.tmpl`, `verify_outcome.html.tmpl`: the link flow D-10 replaces.
- `internal/service/trustcopy.go` and the popup trust block: reused on the page.
- `web/templates/settings.html.tmpl`: the Username row with its live check goes in About you.

### Established Patterns
- Public routes before the gated group in `internal/api/router.go`; the exempt list is pinned by `internal/api/gate_test.go`.
- Text reaches the page through `textContent` or `html/template` escaping only.

### Integration Points
- The popup footer comment count (D-03), the `/?report=ID` link, the 3.2 updates pipeline (D-16), the Phase 4 moderation screen (D-14).

</code_context>

<specifics>
## Specific Ideas

- "should work exactly like how instagram lets you comment and reply" and "you know how the Instagram works it should work exactly like that" (names at sign up).
- "should have an account no matter what like use this website".

</specifics>

<deferred>
## Deferred Ideas

None new in this round. Profiles, follows and stars are Phase 9.

</deferred>
