---
phase: quick-261113-l9x
plan: 01
subsystem: mailer
tags: [smtp, gmail, starttls, mailcheck, policy]
status: complete
requires: []
provides:
  - "internal/mailer SMTP mailer (standard library only) behind the existing Mailer seam"
  - "SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, MAIL_FROM settings"
  - "go run ./cmd/mailcheck, a one email send check"
affects: [cmd/server, README.md, docs/TRUST-MODEL.md, web/templates/legal_body.html.tmpl]
key-files:
  created:
    - internal/mailer/smtp.go
    - internal/mailer/smtp_test.go
    - internal/mailer/smtptest/smtptest.go
    - cmd/mailcheck/main.go
    - cmd/mailcheck/main_test.go
  modified:
    - internal/mailer/loader.go
    - internal/mailer/loader_test.go
    - internal/mailer/mailer.go
    - internal/mailer/logmailer.go
    - internal/mailer/ceiling.go
    - cmd/server/main.go
    - internal/api/handlers/credentials.go
    - go.mod
    - go.sum
    - README.md
    - docs/TRUST-MODEL.md
    - web/templates/legal_body.html.tmpl
    - web/review71_templates_fix_contract_test.go
  deleted:
    - internal/mailer/resend.go
    - internal/mailer/resend_test.go
decisions:
  - "STARTTLS (or implicit TLS on 465) always before AUTH; a defence in depth check of the TLS state sits right before the login"
  - "A half set SMTP group is an error outside development and a warning plus the log mailer in development"
  - "Errors keep their kind and the reply code and drop the server's words and every setting"
plan_head_before: e754b884f483ac91e884b0be0d93cbe7b69630ed
plan_head_after: 08984166
commits: 3
actuals:
  tasks: 3
  commits: 3
---

# Quick 261113-l9x: sign in emails through Gmail, Resend dropped, a send check

Pinalert now sends its four sign in emails through a Gmail account over SMTP with an app password, using only the standard library; the Resend module, mailer and docs are gone, and `go run ./cmd/mailcheck -to <address>` sends one sample code email so the owner can see it in a real inbox.

## Commits

| Task | Commit | Subject |
| --- | --- | --- |
| F1 | 3f1d4f0b | feat(261113-l9x): send sign in emails through Gmail SMTP and drop Resend |
| F2 | 02555231 | feat(261113-l9x): add mailcheck, a command that sends one sample sign in email |
| F3 | 08984166 | docs(261113-l9x): README Gmail section, trust model ceiling lines and the Policy sign in email line |

## Settings

`SMTP_HOST`, `SMTP_PORT` (default 587), `SMTP_USERNAME`, `SMTP_PASSWORD`, `MAIL_FROM` (default `Pinalert <SMTP_USERNAME>`).

- Host set: user name and password are required too, and the error names the missing setting, never a value.
- None set and `ENV=development`: the log mailer, with a warning naming `SMTP_HOST`.
- None set elsewhere: an error naming the three required settings.
- A half set group (for example only `SMTP_PASSWORD`): an error outside development; a warning and the log mailer in development.

## Security shape of the mailer

- Port 465 starts TLS at the first byte. Every other port uses STARTTLS and the send is refused (no AUTH, no MAIL) if the server does not offer it. The test server advertises AUTH PLAIN on the plain connection on purpose, because Go's own `smtp.PlainAuth` would send a password to a localhost server in plain text; our code refuses first.
- TLS 1.2 or newer, certificate chain and host name checked. No `InsecureSkipVerify` anywhere.
- The whole exchange is bounded by the context deadline, or 20 seconds without one; a cancelled context closes the connection.
- The recipient must be one plain address (no display name, brackets, list or line break); CR, LF or NUL in To, From or Subject is refused before any connection. The subject is Q encoded when not ASCII; both parts are UTF-8 quoted-printable; the Message-ID is random at the From domain.
- Errors are one of: context, could not reach the server, could not secure the connection, no encrypted connection offered, try again later (4xx), sign in refused (any AUTH failure), message refused (other 5xx). The reply code is kept, the server's words are not. No password, user name, recipient or code reaches an error or a log line; tests pin this with server replies that repeat the address and the user name.
- No live network call anywhere. Tests use in process fake servers on random 127.0.0.1 ports with a certificate made in the test.

## mailcheck

Usage line: `go run ./cmd/mailcheck -to someone@gmail.com [-kind signup|reset]`

Uses `mailer.LoadMailer("production")`, so the log mailer is never used and missing settings exit 1 naming them. It touches no database. A random 6 digit code from `crypto/rand`, the lifetime from `service.DefaultCredentialLimits.CodeTTL`. It prints `Sent. Check the inbox and the spam folder.` or the failure kind, and echoes nothing the person typed.

## Deviations from Plan

1. **[Rule 3] Exported `mailer.NewSMTPMailerWithRootCAs(cfg, pool)`.** The plan said tests inject a root pool through an unexported hook. That works for `internal/mailer` tests (the unexported `rootCAs` and `forceImplicitTLS` fields), but `cmd/mailcheck` is another package and its test must reach a fake server. The exported constructor only replaces the trust anchors; the chain, the host name and TLS 1.2 are still checked in full. Nothing in the server startup path calls it.
2. **New package `internal/mailer/smtptest`.** A non test file that imports `testing`, shared by the mailer and mailcheck tests instead of duplicating the fake server. Only `_test.go` files import it.
3. **`SMTP_PASSWORD` has its spaces dropped.** Google shows an app password in four groups separated by spaces; pasting it as shown would otherwise fail. This silently changes a secret, so the README says so.
4. **`internal/api/handlers/credentials.go` touched** (not in `files_modified`): one comment said the Resend client may take a minute; it now says the mail send may take up to 20 seconds.
5. **`mailcheck` prints a second line on a setting error** telling the owner which settings to set in the shell, because the loader's own text says "outside ENV=development", which is confusing for a check command.
6. **README owner checklist keeps its numbering.** Steps 1 and 2 (Resend domain) became "do the Gmail steps and run mailcheck" and "check the host allows port 587 or 465", so later step numbers did not move.
7. **TRUST-MODEL.md:** three ceiling lines (655, 665, 667) were updated, not one, with the dated amendment note on the first.
8. **Literal `git grep -n 'resend-go'` is not empty.** It prints 30 lines, all in `.planning/` history (old plans, research and summaries of Phase 01.1 and 7.1), which are left untouched. Scoped with `':!.planning'` it prints nothing (and `go.mod`/`go.sum` hold no match). This is a mismatch with the literal instruction, not a pass of it. `git grep -ni 'RESEND_' -- ':!.planning'` prints nothing.
9. `gofmt -l` lists three files that this task did not touch and that were already unformatted at the base commit (`internal/api/handlers/place_test.go`, `internal/geocode/photon.go`, `internal/store/comments_test.go`). Every touched file is clean.

## Gate

- `go build ./...`: ok. `go vet ./...`: ok.
- `gofmt -l` on all touched files: nothing.
- `go test ./... -p 1 -count=1 -v` on database `q_l9x_a` (created and dropped by the executor): exit 0, 22 packages ok, 6 with no test files, 0 FAIL; 2476 top level test PASS lines, 6861 PASS lines including subtests, 3 SKIP (TestGeocodeLive, TestOfficialLive, TestScreeningFixture). `go test -race` on `internal/mailer` and `cmd/mailcheck` passed.
- `node --test web/*.mjs`: 1627 tests, 1627 pass, 0 fail, 0 skipped.
- Wording check: prints nothing. Em or en dashes: none added; `cmd/server/main.go` 6 to 5, `README.md` 14 to 11, `loader.go` and `loader_test.go` 2 to 0, every new file 0.

## Deploy day note

The Policy line now names Google (Gmail). The Policy "Last updated" date and `service.CurrentTermsVersion` were NOT bumped; the parked deploy day bump (README owner step 9) now also covers this line.

## Known Stubs

None.

## Threat Flags

| Flag | File | Description |
| --- | --- | --- |
| threat_flag: outbound-credential | internal/mailer/smtp.go | The server now holds a mail account app password and sends it over TLS to the configured SMTP host. Mitigated: TLS before AUTH, certificate and name checked, secrets kept out of errors and logs. The README says to keep it as a host secret and how to revoke it. |

## Self-Check: PASSED

Created files exist, the three commits are ancestors of HEAD, and the resend files are deleted.
