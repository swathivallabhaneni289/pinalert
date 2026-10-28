---
sketch: 010
name: password-sign-in
question: "Does the approved Phase 7.1 design (a Create account and Sign in card, a typed code, Forgot password, the required About you step with a Username, the Settings Username row) read clearly on a phone and a laptop, in light and dark, with the Phase 5 globe behind the card?"
winner: "none yet, waits for the owner viewing"
tags: [login, sign-in, create-account, password, typed-code, username, about-you, settings, phase-7.1, phone, laptop, light-dark]
---

# Sketch 010: Password sign in and the required About you (Phase 7.1)

## Design question

Phase 7.1 replaces the emailed link with an email and a password, one typed 6 digit code at sign up, a typed code for a
forgotten password, and a required step 4 (Username, Name, Home area). The design is written down and approved in
`.planning/phases/07.1-password-sign-in-and-usernames/07.1-UI-SPEC.md` (its copy table, the P71 rows, is the exact wording).
Phase 7.1 execution waits for the owner to look at it (07.1-CONTEXT status line, ROADMAP Phase 8 scope note 6). This sketch
is that viewing. It is a planning artifact: nothing here is app code.

## How to open it

Open `index.html` in a browser (double click it, or `open index.html`). It is one self contained file with no build step,
and it works from a file or from any static server (use a port other than 8080 and 8090). Only the globe needs a network
connection; without one the page keeps its plain background.

- **A wide window** (900px and more) shows the mockup panel on the left and the page on the right. **Width** switches the
  page between a 390px phone frame and the laptop width. **Hide controls** gives the page the whole window.
- **A narrow window** (a real phone) shows the page itself, with the panel folded at the top under **Show controls**.
- **Theme:** Light, Dark, or Device (follows the device setting, like the real login page).
- **Jump to a screen** goes straight to any surface. Every surface can also be reached by clicking through the page itself.
- Address parameters behind the switches: `view=phone|laptop`, `theme=light|dark`, `go=<screen>`. The address never holds
  an email, matching the new rule (D-05). Screens for `go`: `create`, `signin`, `forgot`, `reset-done`, `signin-wrong`,
  `signin-locked`, `code-create`, `code-reset`, `code-wrong`, `code-expired`, `code-tries`, `code-refused`, `done-oldlink`,
  `done-switch`, `done-alldevices`, `done-deleted`, `step4-new`, `step4-empty`, `step4-old`, `settings`,
  `settings-username`, `settings-wait`, `settings-name`, `settings-home`, `email-signup`, `email-reset`, `email-exists`,
  `email-noaccount`.

The panel is the mockup's own and is not part of the app. It has the things a real run makes you wait for or cannot
trigger on purpose: **Skip the resend wait**, **Expire the code now**, **Next answer from the server** (Offline, Server
problem, Device limit, Account locked, Code cap for an hour; each applies to the next request only), and **Browser checks
before sending** (Off lets you see the server's own sentences, such as "Use at least 8 characters.", that the browser
check normally stops first).

## The surfaces (every one in the UI-SPEC, reachable by clicking)

| Surface | How to reach it | Jump |
|---------|-----------------|------|
| The sign in card, with the Create account and Sign in choice | opens on Create account (the default); press Sign in | Create account, Sign in |
| Create account, then the code screen | type an email and a password of 8 or more characters, press Email me a code | Code, New account |
| The code screen: six digit entry, Resend with its wait, Change email | on the code screen; Resend counts down from 45s; Change email returns to the page you came from | Code, New account |
| The wrong, expired and too many tries states | type any code other than 123456; wait for 10 minutes or press Expire the code now; five wrong codes in a row | Wrong code, Expired code, Too many tries |
| A refused Resend, shown inside the panel | with Next answer set to Device limit or Code cap, press Resend code | Resend refused |
| Sign in, "Email or password is wrong." and the old account line | press Sign in, type a wrong password; the old account line shows on every wrong try | Wrong email or password |
| A locked sign in | five wrong tries for one email, or an email starting with `locked` | Locked account |
| Forgot password, then the code and the new password | press Forgot password on the Sign in panel, then Email me a code, then the code and a new password | Forgot password, Code, Reset password |
| After a reset | finish a reset: the Sign in panel returns with "Password set. Sign in to continue." | After a reset |
| The one time notice for an old link account, and the old link landing line | the line above the title, opening on Create account | Signed out by the switch, Old emailed link |
| The built lines for Log out of all devices and Delete my account | the same line, opening on Sign in or Create account | Logged out everywhere, Account deleted |
| Step 4 About you, required, no Skip, Continue switched off until all three are good | finish Create account with the code 123456 | New person, Nothing filled in |
| Step 4 for an old account filling in what is missing | create an account with an email starting with `old` | Old account filling in |
| The Username live check: available, taken, wrong characters, kept for official use | type in the Username field on step 4 or in the Settings window | any step 4 or Settings jump |
| Settings, About you: Username, Name, Home area | Settings jump (the real page is reached from the person menu or the gear) | Settings |
| The Username window, with "This is your username now." and the 7 day wait | press the Username row; change it and save, then open it again | Username window, Changed 3 days ago |
| The Name and Home area windows with their empty save sentences | press the Name or Home area row, save with nothing | Name window, Home area window |
| The four emails | the emails the code screens speak of | Code to create, Code to reset, Account exists, No account |
| One row of four outlined buttons under the card: Try the demo, Low data view, Disclaimer, Policy (1e, default 22). A two by two grid of equal buttons on the phone, one centred row on the laptop, no box behind it, no "Text only, no map" line | under every sign in card; resize or switch Width to see both arrangements | any sign in screen |
| Low data already on: the second button reads "Turn off low data view" and one line above the row says "Low data view is on. It opens after you sign in." The globe is off | press Low data view (it turns on in place; press it again to turn off), or use the panel group below | panel group "Low data view already on for this device" |

## What the fake server does

- **Code:** `123456` works. Any other code is wrong. Five wrong codes in a row lock that code until a new one is sent. A code
  that has expired is refused even if it is 123456. Resend is possible every 45 seconds, and the code lasts 10 minutes.
- **Sign in:** `priya@example.com` with `pinalert-demo` works. An account you create here, or a password you reset here,
  works too. Five wrong tries for one email lock it (the wait is 5 minutes). An email starting with `locked` is locked at
  once. The sentence is the same for an unknown address, a wrong password and an account with no password.
- **Old link account:** an email starting with `old` opens step 4 as an old account, with the Name and Home area already
  filled in and the lead "Pinalert now asks everyone for a username, a name and a home area. Fill in what is missing."
- **Passwords:** a short one, a common one (password, 12345678, qwerty123 and similar) and the email address itself are
  refused with their P71-C17 sentences.
- **Usernames:** `ravi`, `ravi.k`, `anjali.r`, `kiran_77` and `meena.s` are taken. `police`, `p0lice`, `police.help`, `ndma`,
  `imd`, `govt`, `admin` and similar (the UI-SPEC lists) are kept for official use. `rude.name` passes the check and is
  refused on save by the text screening. `justtaken` shows available and is taken by the time it is saved. Capitals fold to
  lowercase as you type and a leading `@` is dropped.
- **Home area search:** Benz Circle, Gayathri Nagar, Indiranagar, Koramangala, Andheri West, T Nagar, Salt Lake, Banjara
  Hills (type three letters or more). A home area counts as filled only when a place is picked or already stored.
- **Not simulated:** the real server sends no usable code to an address that has no account (Forgot password) or already has
  a password (Create account); the screen is the same, and here `123456` works for any address. A signed in person who opens
  the sign in page is redirected in the real app; here the page after sign in has a button to start again.

## The look

Copied from the built app, not restyled: the tokens, type and spacing from `web/static/css/main.css`, the card and field
from `auth.css` (as it is on disk now, not 009's older copy: the field border is the muted text
colour), `.btn` and `.inline-error` from `chrome.css`, step 4 and the
laptop window from `welcome.css`, the edit windows from `window.css`, and the Settings list from `places.css`. The Light and
Dark switch works through `?theme=` and `prefers-color-scheme`, as in 009. Icons are the project's own shapes drawn as inline
SVG masks; the new `lock` is the Lucide shape. The globe is 009's loader (placement A, the realistic Earth), with its speed and credit unchanged. Owner change, 2026-10-29: the Earth shows at full strength (no dimming), on a phone it is whole and centred above the card (its box reserved in CSS, 140px to 300px, sized so the main button stays on the first screen) with the card overlapping only its lower fifth, no longer twice the width behind the card, and the card and the row of buttons under it are frosted glass while the globe is on (the surface or background colour at 65 percent with a 4px blur, solid without backdrop-filter support or with reduced transparency; at 65 percent the text stays at least 5 to 1 even over pure black or pure white, and measured over the Earth it was 5.6 to 1 or more). The laptop globe keeps its
size. The row under the card is the built outlined button look (44px, 8px corner, 1px border, Label 14px, weight 400, no underline, padding 0 12px) drawn as the UI-SPEC section 1e says: the old grey box and `.login-column` background are gone, so the globe shows between the buttons. The sign in panels swap in place with the `hidden` attribute so the globe element is never touched.

## Defaults to look at

Copied from "Defaults to confirm at the viewing session" in the UI-SPEC. Nothing was asked; each line is built as written
unless you veto it when looking at the finished screens.

**Orchestrator defaults**

1. **Password minimum 8 characters, at most 128.** A password on the common list is refused; there are no rules about
   capitals, digits or symbols. A Show/Hide control and the password manager attributes are in. The stricter option is NIST
   SP 800-63B-4's **15 characters** when a password is the only sign in factor (research recommended it). Eight was chosen
   because the owner asked for an easy sign up. The blocklist research sized for 15 character passwords must now cover common
   8 to 14 character passwords (planner note). In this mockup the rule line reads "At least 8 characters."; at 15 it would
   read "At least 15 characters." and nothing else on the screen changes.
2. **Option X for old accounts.** The change ends the sessions of every account with no password. They come back through
   Create account with the same email (one code, then signed in, then the fill in). The alternative, Option Y, keeps their
   sessions and sends them to a set a password step with no code; it needs two more signed in routes and lets a stolen
   cookie set a password that outlives revocation.
3. **Name and Home area can be changed but not emptied.** Settings loses Remove home area; empty saves are refused with a
   sentence. The alternative keeps the old API clears and lets the gate send the person back to step 4.
4. **Every number the research proposes**, in plain words:
   - A code has 6 digits and works for **10 minutes**, once.
   - A person has **5 tries** per code; asking for a new code replaces the old one.
   - **Resend** is possible every **45 seconds**, and at most **5 codes per email address per hour**.
   - A code can only be used in the **same browser** that asked for it.
   - Wrong passwords: **5 free wrong tries per account**, then a wait of **5 minutes**, doubling with each further wrong try
     up to **60 minutes**; the count is forgotten after **30 minutes** without a wrong try or after a successful sign in or
     reset. A reset also clears the wait. The message is the same for an address with no account.
   - Per device: about **8 quick sign in tries**, then one more every **minute**. Per network address: a much looser limit
     (300), switched on only when the host passes the real address.
   - Code requests per device: **5 quick**, then one more per **minute**.
   - The whole site stops sending mail at **90 emails a day** (the free plan's limit is 100).
   - Username check: **10 quick**, then one per **second**. Username save: **5 quick**, then one per **10 seconds**; a name
     can be changed **once every 7 days** after the first.
   - Username: **3 to 20 characters**; letters, numbers, underscores, full stops; at least one letter; no full stop at the
     start, the end, or twice in a row; stored in lowercase. Reserved words: police, ndma, ndrf, sdma, sdrf, govt,
     government, official, pinalert, moderator, administrator, emergency, fire, ambulance (kept even with look alike
     digits, and as a start or a whole word between separators), and exact only: imd, gov, mod, admin, help, support, 100,
     112.
   - Sign in stays valid for the cookie's existing **1 year**.

**Placement and look**

5. The **default choice** is Create account, so a new person lands on it; returning people press Sign in. The alternative is
   Sign in first, or remembering the last choice on the device.
6. The card title is **Pinalert**, with the two choices directly under it. The alternative is a title that follows the
   choice (Create account, Sign in).
7. The chosen choice is shown by a page coloured fill and semibold weight inside a grey track.
8. Forgot password is **two screens**, code and new password together on the second. The alternative is three screens: email,
   code, new password.
9. The Username window in Settings instead of the sketch's inline editor (D1).
10. The `@` shown at the left of the Username input.
11. The text-only **Show / Hide** on password fields instead of an eye icon.
12. Continue is **switched off until all three are good** (as sketch 008), with a press explaining what is missing.
13. The one **old account line** under every wrong sign in, and the one time notice for the browser the change signed out.
    Both say to use Create account with the same email, because that is the one step route.

**Behaviour**

14. A **reload on the code screen** returns to the code screen (the server shows it from the session). The fallback is the
    first panel with the email filled in. (The Code, New account jump is the screen a reload would show.)
15. After a **reset** the person signs in again (no automatic sign in), and every other device is signed out.
16. **No Change password row** in Settings. Log out and use Forgot password.
17. A username the text screening cannot judge is **accepted as pending** with no message; nothing shows a username until
    Phase 8.
18. `pinalert.pushIntent` is **left alone** by sign in and sign out in this phase (the open question in the seams file). The
    server deletes the replaced session's push subscriptions so the next person on a shared phone does not get the previous
    person's alerts. Say if the owner wants the flag cleared at log out.
19. The **Policy** gets the password hash, public username, name and home area named in "What Pinalert keeps" and a new "Last
    updated" date; the terms version moves with it, so every existing account passes the terms step again in the same fill
    in visit.
20. Emails carry **no link and no address to click**; the code must be typed in the same browser.
21. Owner steps before public sharing: a verified Resend sender domain (the sandbox sender only delivers to the Resend
    account holder), and a first real iPhone check of a typed code inside the installed app.
22. **One row of buttons under the card** (1e): Try the demo, Low data view, Disclaimer, Policy as equal outlined buttons, a two
    by two grid on the phone and one row on the laptop, with no box behind them and no "Text only, no map" line. When Low
    data is already on, the second button reads "Turn off low data view" with one status line above the row.

## Noticed while building it (for the planner)

- **A pasted code and `maxlength="6"`.** The UI-SPEC says a pasted `482 913` becomes `482913` and also asks for
  `maxlength="6"`. A browser cuts a paste to six characters before the script can pick the digits out, so `482 913` became
  `48291`. The mockup cleans the paste first (a `paste` handler) and keeps `maxlength` for typing. The build needs the same.
- **Error line size on the card.** The built `.inline-error` is 16px; the UI-SPEC Typography row says status and error
  lines on the card are Label 14px. The mockup uses 14px on the card (a selector that wins over `.login-card p`) and the
  built 16px on step 4 and in the windows. Say if all error lines should be one size.
- **The switched off look.** chrome.css gives `aria-disabled` buttons muted text; the UI-SPEC wants full contrast text on the
  card and on step 4. The mockup overrides it once for every `aria-disabled` button. The build must not rely on chrome.css.
- **Tab order (D10).** The choice control is one Tab stop, then the email field, then the buttons, then the row of four
  buttons under the card (Try the demo, Low data view, Disclaimer, Policy) and last the globe credit. 05-UI-SPEC section 5c still says the email field is first.
- **The globe credit links** are underlined in sketch 009. Here they are semibold with no underline at rest (underline on
  hover and keyboard focus only), to follow the no underlined text rule.
- **Settings Name window label.** The built window says "Your name"; the mockup says "Name", like step 4 and the Settings
  row (P71-C4). **Save home area** is pressable and answers "Choose a home area." as the UI-SPEC says; the built button is
  switched off until a place is picked.
- **The Username field is a box holding the `@` and the input.** The box is 44px, like the Name and Home area fields, and a
  press anywhere in it, the `@` included, puts the cursor in the input. The input itself is 42px inside the box, which the
  rules checker reports as a small target; the build can instead draw the box edge with the input's own border and put the
  `@` in a prefix inside it.

## Left out or changed in this mockup only

- The step panel of the laptop first-time flow also carries a one sentence tagline under the wordmark in the built page. It is
  left out here because of the no tagline rule; the step list stays, as plain rows (the built rows are buttons that jump to
  a step, which this sketch does not draw).
- Steps 1 to 3 and 5 of the first-time flow, the phone bottom bar, the other Settings sections and the app itself are not
  drawn. Where they would come next, a plain page says so.
- The emails show the code `123456` (the UI-SPEC's `482913` is sample text) so the mail and the fake server agree.
- The line above the title (the notices) is dropped once the card moves to the code or Forgot password screen. The UI-SPEC
  does not say; in the real page it would stay until the next page load.

## Checks run

Run on the 2026-10-25 build, with Playwright (Chromium and its WebKit build), pages served from a scratch folder on a port
in 8770 to 8789 and then stopped. Screenshots stayed in the session scratchpad, never in the repo.

- Clicked through, in Chromium at 390 by 844: Create account (browser check, common password, busy label), the code screen
  (paste, wrong, five wrong, the lock, Change email keeping the email and the password, a second code), step 4 (the rule
  when empty, Continue off and its press, folding and the dropped `@`, taken, three kinds of official, available, clearing
  the home area, a place pick, the lost race and the text screening on save, Continue), Sign in (wrong, five wrong then the
  lock, Show and Hide), Forgot password (email copied, the code and a common new password, the success line and the focus),
  an expired code, Resend (Sending, sent, refused, the hour cap), the forced answers, Browser checks Off, Settings (all three
  windows, the 7 day wait after a save, Escape, focus returning to the row).
- The panel and the frame in Chromium and WebKit, from `http://` and from `file://`: jumps, theme, Phone and Laptop, a forced
  answer, Hide controls, and no email in the address.
- The page as the top level document at 320, 360 and 390 in both engines: no sideways scroll, the two choice labels on one
  line, jumps and the arrow, Home and End keys on the choice.
- `.planning/sketches/shared/rules-check.js` on 23 screens at 390 and 1280, light and dark: no contrast, overflow, dash,
  emoji, gradient or weight finding, and no small target except the 42px Username input described above. What it still
  reports is copied as built: the 12px gaps and padding, the 12px
  radius and window shadow of the windows and Settings boxes, the 12px step numbers, the 2px bar under a top bar link, the
  45 percent dim behind a window, the dark step panel colours and the mixed muted colour of the Settings second lines.
- The row under the card (2026-10-27 change), in Chromium and WebKit at 320, 360, 390, 1280 and 1440: no sideways scroll, the order
  Try the demo, Low data view, Disclaimer, Policy, every button 44px high, Label 14px at weight 400 with no underline, no
  box, border or background on the row, 16px below the card, a two by two grid of equal widths with 8px gaps as wide as the
  card at 320, 360 and 390, and one centred row with 8px gaps at 1280 and 1440. With Low data on, the second button reads
  "Turn off low data view" (two lines at 320, no overflow), the status line shows and the globe is off; the panel toggle and
  the page button stay in step. The rules checker reports only the 12px side padding the UI-SPEC asks for on these buttons.
- Globe: turning in Chromium behind the laptop card (light) and the phone card (dark) from `http://`, and turning in both
  Chromium and WebKit when `index.html` is opened straight from the file.

Not checked: real Mac Safari and the iPhone simulator (the owner's look is the check that counts, and Playwright's WebKit has
passed layouts the real Safari showed broken before), 200 percent text, forced colours, a screen reader, a real one time code
suggestion on iOS, a slow network.
