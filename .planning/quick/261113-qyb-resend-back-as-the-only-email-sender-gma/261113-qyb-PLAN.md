---
phase: quick-261113-qyb
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/mailer/resend.go
  - internal/mailer/resend_test.go
  - internal/mailer/loader.go
  - internal/mailer/loader_test.go
  - internal/mailer/mailer.go
  - internal/mailer/logmailer.go
  - internal/mailer/ceiling.go
  - internal/mailer/smtp.go
  - internal/mailer/smtp_test.go
  - internal/mailer/smtp_kinds_test.go
  - internal/mailer/smtp_limits_test.go
  - internal/mailer/smtptest/smtptest.go
  - internal/mailer/guard_test.go
  - internal/mailer/redact_test.go
  - internal/mailer/loader_host_test.go
  - internal/mailer/loader_password_test.go
  - internal/mailer/loader_required_test.go
  - internal/api/handlers/credentials.go
  - cmd/server/main.go
  - cmd/mailcheck/main.go
  - cmd/mailcheck/main_test.go
  - cmd/mailcheck/hint_test.go
  - cmd/mailcheck/imports_test.go
  - go.mod
  - go.sum
  - README.md
  - docs/TRUST-MODEL.md
  - web/templates/legal_body.html.tmpl
  - web/review71_templates_fix_contract_test.go
requirements: [QUICK-261113-qyb]
must_haves:
  truths:
    - "Pinalert sends its four sign in emails through Resend only; the Gmail (SMTP) sender, its settings and its tests are gone"
    - "With ENV=development and RESEND_API_KEY unset the server still prints each email to its log, and outside development it refuses to start without RESEND_API_KEY"
    - "mailcheck sends one sample code email through Resend, touching no database, so the owner can see a real email in the inbox of their own Resend sign up address today, before any domain exists"
    - "The Policy names Resend as the company that sends the sign in emails"
    - "The one place the code lifetime lives (mailer.CodeLifetime) and its pin stay"
---

# Quick 261113-qyb: Resend back as the only email sender, Gmail removed

## Why

On 2026-11-13 the owner first chose Gmail (quick 261113-l9x). Then it came out that Render's free plan blocks the mail ports Gmail needs (Render changelog, live since 2025-09-26), and that apps normally send codes through an email service from their own domain. Asked "keep the gmail or resend?", I recommended Resend plus a domain (about $10 a year, bought on deploy day); the owner said "we will have only one", then "sure" to Resend. So Resend is the one sender, over HTTPS (port 443), which works on any host.

Base commit for the old Resend code: `e754b884` (the l9x plan commit, right before l9x changed any code). Between `e754b884` and HEAD only l9x commits touched these files, so restoring a file from `e754b884` loses nothing else. Check that with `git log --oneline e754b884..HEAD -- <file>` before restoring each one.

## Task R1: the Resend sender back, Gmail removed (one commit)

1. Restore from `e754b884` exactly (`git checkout e754b884 -- <path>`): internal/mailer/resend.go, internal/mailer/resend_test.go, internal/mailer/loader.go, internal/mailer/loader_test.go, internal/mailer/logmailer.go, internal/mailer/ceiling.go, internal/api/handlers/credentials.go, go.mod, go.sum. Then `go mod tidy` (resend-go v3.17.0 comes back; no other module change). Restored Go comments may hold em dashes from before; leave Go code comments as they were except where you touch a line.
2. cmd/server/main.go: put back the three Resend comment blocks from `e754b884` (the ceiling comment near line 258, the LoadMailer comment near line 354, the "unlike SESSION_SECRET and RESEND_API_KEY" comment near line 387). The rest of the file stays as HEAD has it (verify with a diff that only those comments differ from `e754b884`).
3. internal/mailer/mailer.go: the package comment names Resend again (as at `e754b884`), and KEEP `CodeLifetime` (and its `time` import) exactly as HEAD has it. internal/service/credentials.go and internal/service/codelifetime_test.go stay as HEAD has them.
4. Delete: internal/mailer/smtp.go, smtp_test.go, smtp_kinds_test.go, smtp_limits_test.go, the internal/mailer/smtptest/ folder, guard_test.go, redact_test.go, loader_host_test.go, loader_password_test.go, loader_required_test.go. Before deleting guard_test.go, read it: if it guards anything other than the SMTP test only hooks, keep that part working against the Resend code; otherwise delete it.
5. The loader: keep the `e754b884` behaviour (RESEND_API_KEY set: Resend, RESEND_FROM optional with the sandbox default `Pinalert <onboarding@resend.dev>`; unset in development: log mailer with the warning naming RESEND_API_KEY; unset elsewhere: an error naming RESEND_API_KEY). Add one small exported helper so mailcheck can tell the missing key apart, for example `var ErrMissingAPIKey = errors.New(...)` wrapped by the loader error, with errors.Is; add a loader test for it. The error text never holds a value.

## Task R2: mailcheck on Resend (one commit)

cmd/mailcheck/main.go keeps its shape (flags, `isPlainAddress`, `sampleCode`, the `load` seam, `mailer.CodeLifetime`, the success line "Sent. Check the inbox and the spam folder.").
- The package comment and usage name RESEND_API_KEY (required) and RESEND_FROM (optional). Say that without a verified domain the default sender only delivers to the email address the Resend account was made with, so `-to` must be that address until the domain is added.
- The hint after a load error: only when the error is the missing key (errors.Is with the new helper), print "mailcheck: set RESEND_API_KEY (RESEND_FROM is optional) in this shell, then run it again."
- Tests: TestSendsOneSampleEmail and TestFailureShowsOnlyTheKind use a recording fake `mailer.Mailer` through the `load` seam (no SMTP fake, no network). TestMissingSettingsExitNonZeroAndNameThem names RESEND_API_KEY. Drop TestHalfSetSettingsExitNonZero and TestMissingPasswordExitsNonZero (no such groups now). hint_test.go: the hint shows for the missing key and not for a send failure. imports_test.go (TestMailcheckStaysSmall) stays; adjust only if its allowed list named SMTP packages.

## Task R3: docs and the Policy (one commit)

- web/templates/legal_body.html.tmpl and web/review71_templates_fix_contract_test.go: restore from `e754b884` (the line "Sign in emails: Resend sends them, so it sees your email address and the email, including any code."). The Policy date and CurrentTermsVersion are NOT bumped (deploy day item).
- README.md: restore the env lines at the top (around lines 33 to 35 and 52 to 55 at `e754b884`) and the "Email delivery (Resend)" section from `e754b884`, replacing the Gmail section. While restoring, write every em or en dash in those restored lines as plain words or a comma. Add to the Resend section:
  - Owner check today, before any domain: in your own Terminal app,
    ```
    cd ~/code/pinalert
    read -s RESEND_API_KEY     # paste the key, nothing is shown, press Return
    export RESEND_API_KEY
    go run ./cmd/mailcheck -to YOUR_RESEND_SIGNUP_EMAIL
    exit
    ```
    The `-to` address must be the one the Resend account was made with until a domain is verified. Use a new API key named Pinalert, never one from another project, and never paste it into a chat or a file.
  - On deploy day: buy a domain, add and verify it in Resend (Domains, Add Domain, then the DNS records), set RESEND_FROM to an address at it (for example `Pinalert <codes@YOUR_DOMAIN>`), then run mailcheck to an address that is not the sign up one. If this Resend account already holds another project's domain and the free plan's domain limit is reached, make a separate Resend account for Pinalert.
  - Resend sends over HTTPS (port 443), so hosts that block mail ports (Render's free plan blocks 25, 465 and 587) are fine.
  - If the key leaks: delete it in the Resend dashboard (API Keys) and set a new one.
  Keep the placeholders YOUR_RESEND_SIGNUP_EMAIL and YOUR_DOMAIN; never write a real looking address.
- docs/TRUST-MODEL.md: restore the three lines (655, 665, 667) from `e754b884` and add at the end of the ceiling line one short dated note: "Amended 2026-11-13: the emails went through Gmail for one day (quick 261113-l9x) and are back on Resend (quick 261113-qyb)."
- PROJECT-NOTES.md and any other doc outside .planning: only if a line names Gmail or SMTP as the sender.

## Gate and rules

- Full gate: go build ./..., go vet ./..., `go test ./... -p 1 -count=1 -v` on a fresh database q_qyb_a (only TestGeocodeLive, TestOfficialLive, TestScreeningFixture may skip), `node --test web/*.mjs`. Then these print nothing: `git grep -n -i 'smtp' -- . ':(exclude).planning'`, `git grep -n 'smtptest'`, `git grep -n -i 'gmail' -- . ':(exclude).planning' ':(exclude)cmd/seedprofiles'` except the TRUST-MODEL amendment note. `git grep -n 'resend-go'` shows go.mod, go.sum and resend.go only.
- No live network call to Resend or any mail server in any test or check. Never ask for or use a real API key. Made up example.com people only.
- Databases: only q_qyb_a, created and dropped by you. Never pinalert_test, pinalert, p9_view, p31_4_uat, p34_uat, p3_view, p4_uat or any database you did not create. Confirm DATABASE_URL and PGDATABASE are empty, then export DATABASE_URL in the same Bash call.
- Ports: never 8080, 8090, 8776, 8778, 8781, 8782 or 8797, and never touch their processes. Use 8794 if you must start a server (you should not need to).
- Helper files only in /private/tmp/claude-501/-Users-swathivallabhaneni-code-pinalert/2574caf2-6d41-4dbd-95b5-bd3c870af095/scratchpad/qyb-exec/. No images in the repo. No Python in any form, not even --version. No em or en dashes in new prose or commit messages. No migration. Keep the swag @license.name line "Unlicensed (all rights reserved)".
- Path limited commits listing every changed file (`git commit -- <paths>`; deleted files listed too), trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never stage .planning/STATE.md, ROADMAP.md or REQUIREMENTS.md. Never push. The wording check `git grep -niE 'portfoli[o]|recrui[t]|intervie[w]|job[- ]?appli[c]|hirin[g]|linkedi[n]' | grep -v LinkedInOrder` prints nothing.
- SUMMARY at .planning/quick/261113-qyb-resend-back-as-the-only-email-sender-gma/261113-qyb-SUMMARY.md, committed alone and last, listing the gate numbers.
