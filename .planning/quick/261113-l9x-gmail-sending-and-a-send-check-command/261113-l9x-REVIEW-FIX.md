---
phase: quick-261113-l9x
fixed_at: 2026-11-13T00:00:00Z
review_path: .planning/quick/261113-l9x-gmail-sending-and-a-send-check-command/261113-l9x-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 12
skipped: 0
status: all_fixed
---

# Quick 261113-l9x: Code Review Fix Report

**Fixed at:** 2026-11-13
**Source review:** `.planning/quick/261113-l9x-gmail-sending-and-a-send-check-command/261113-l9x-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (WR-01, IN-01 to IN-11)
- Fixed: 12
- Skipped: 0
- Left as a note on instruction: IN-12 (see "Noted, out of scope")

Gate, run in the isolated worktree (`.claude/worktrees/agent-a273432bd58f8812a`, not the main checkout) on the scratch database `q_l9x_fix` (created for this run, dropped afterwards; `DATABASE_URL` and `PGDATABASE` were empty before it was set):
- `go build ./...` and `go vet ./...`: clean. `gofmt -l` on every touched Go file: nothing.
- `go test ./... -p 1 -count=1 -v`: exit 0, 22 packages `ok`, 6 with no test files, 0 `--- FAIL`; 2493 top level `--- PASS` lines, 6917 `--- PASS` lines with subtests; exactly three `--- SKIP`: TestGeocodeLive, TestOfficialLive, TestScreeningFixture.
- `go test -race -count=1 ./internal/mailer/... ./cmd/mailcheck/...`: ok (smtptest has no test files).
- `node --test web/*.mjs`: 1627 tests, 1627 pass, 0 fail, 0 skipped.
- Wording check (`git grep -niE ...| grep -v LinkedInOrder`): prints nothing. No em or en dash on any added line.
- Compared with the SUMMARY gate (2476 and 6861 on `q_l9x_a`), the new counts are the new tests: 17 more top level and 56 more with subtests.

No network call was made to any mail server; every test uses the in process fake server on a random 127.0.0.1 port, and the only passwords are made up test values.

## Fixed Issues

### WR-01: The app password printed under %v and %+v of the mailer or its config

**Files modified:** `internal/mailer/smtp.go`, `internal/mailer/smtp_test.go`, `internal/mailer/redact_test.go` (new)
**Commit:** f62fd234
**Applied fix:** `SMTPConfig` and `smtpMailer` get `String`, `GoString` and `Format` methods (value receivers, so a pointer and a copy are both covered). The text shows the host and port and says the user name, password and sender are redacted. Two things went beyond the review's snippet, both found by the new test:
- `Format`. A `Stringer` is asked only for `%v %s %q %x %X`; `%d` walked the fields and printed `%!d(string=PASSWORD)`. `Format` closes every verb and takes precedence in fmt; `String` and `GoString` stay as asked.
- `smtpMailer.cfg` is now a `*SMTPConfig`. The review said a ceiling or a service holding the mailer prints only an address; that is true for `%v`, but under `%s` (and `%q`) fmt hits a bad verb on the pointer inside an unexported field, prints the pointee and walks into the config by value. The first run of the test failed on exactly that (`NewSendCeiling` printed with `%s`). With the config behind a pointer, nested printing shows an address.
**Tests:** `TestSMTPConfigAndMailerNeverPrintThePassword` formats eight values (config, pointer, the mailer from `LoadMailer`, the mailer by value, the mailer inside `NewSendCeiling`, a struct holding both, a slice and a map of configs) with `%v %+v %#v %s %q %x %d %T`, `Sprint` and `Sprintln`, and asserts the password, the user name and the sender are absent. `TestSMTPConfigStringKeepsTheHost` pins that the redacted text still names host and port. Flip checked: with `Format` renamed the `%d` cases fail; with the config held by value the ceiling `%s` and `%q` cases failed.
**Other types that hold the password, checked:** `smtptest.Login` holds the password the fake server received. It is test only (guarded by IN-03) and only ever holds the made up test passwords, so it was left as is. `smtp.PlainAuth` in `deliver` is a local value that is never stored or printed. The handler request bodies (`credentials.go` `Password` fields) hold a visitor's own password, a different secret, and were not in scope. A struct that holds an `SMTPConfig` by value in an unexported field would still print it; nothing does, and `smtpMailer` no longer does.

### IN-01: A refused STARTTLS was labelled by its reply code, not as a TLS failure

**Files modified:** `internal/mailer/smtp.go`, `internal/mailer/smtptest/smtptest.go`, `internal/mailer/smtp_kinds_test.go` (new), `README.md`
**Commit:** c6620295
**Applied fix:** At the TLS stage a reply from the server (454, 502, 554 and so on) now returns `errSMTPNoTLS` with the reply code. Its wording is now "the mail server did not start encryption", and the same kind is returned when the server offers no STARTTLS at all and from the TLS state check, so a refused and a missing STARTTLS are one kind. A handshake failure (untrusted certificate, wrong name, old version) stays `errSMTPTLS`. The mailer itself logs nothing; the error reaches the log through `service: sending auth email` and the handler that logs it, with the kind and the reply code and none of the server's words. The README list of failure kinds names the new wording.
**Tests:** `TestSMTPRefusedSTARTTLSIsAnEncryptionFailure` (454, 502, 554 and no STARTTLS offered): the kind, the wording, the reply code, no other kind matches, no AUTH, MAIL, RCPT or DATA command and no login follow, and the server's words are not in the error or the log. New smtptest option `STARTTLSReply`. Flip checked: with the new case disabled, the three reply cases failed.
**Decision to confirm:** "refused or missing" was read as one kind, so the old wording "does not offer an encrypted connection" is gone.

### IN-02: "Refused the sign in" also covered a missing PLAIN and a dropped connection

**Files modified:** `internal/mailer/smtp.go`, `internal/mailer/smtptest/smtptest.go`, `internal/mailer/smtp_kinds_test.go`, `README.md`
**Commit:** 767999e5
**Applied fix:** Two new kinds. A server that offers no AUTH PLAIN after TLS gets `errSMTPNoAuth` ("the mail server offers no password sign in"). A failure at the sign in step with no reply from the server, such as the connection dropping, gets `errSMTPAuthLost` ("the mail server dropped the connection during sign in"). Only a reply at sign in (535, or a 4xx there) stays `errSMTPAuth`, so the existing 454 at sign in test still holds. The README kinds list names both.
**Tests:** `TestSMTPSignInFailureKinds` pins all three apart (535, a server offering only LOGIN and XOAUTH2, and a server that closes the connection on AUTH), that no other kind matches, no message is started, and no server words leak. New smtptest options `AuthMechanisms` and `DropAtAuth`.

### IN-03: The two test only exports were kept out of production by convention only

**Files modified:** `internal/mailer/guard_test.go` (new), `internal/mailer/smtp.go`, `internal/mailer/smtptest/smtptest.go` (two comments)
**Commit:** f332fe27
**Applied fix:** `TestTestOnlyHooksStayOutOfProductionCode` runs `go list` over `./...` from the module root, and fails if the non test files of any package outside `internal/mailer` import `internal/mailer/smtptest` or mention `NewSMTPMailerWithRootCAs` in code (found by parsing, so a comment does not count). `go list` skips dot directories, so worktrees are not walked. `TestGuardFindsAViolation` feeds it a made up package to show it is not vacuous. Flip checked against a real throwaway file in `cmd/mailcheck` that referenced the constructor: the guard failed and named it; the file was deleted and is in no commit. The two doc comments now name the test.

### IN-04: TLS 1.2 or newer and the 20 second limit had no test

**Files modified:** `internal/mailer/smtp.go`, `internal/mailer/smtptest/smtptest.go`, `internal/mailer/smtp_limits_test.go` (new)
**Commit:** 4034b82c
**Applied fix:** smtptest option `MaxTLSVersion` (server side, both STARTTLS and implicit TLS; below 1.2 it also lowers the server minimum so it is a working old server). The mailer gets an unexported `defaultTimeout` field; zero means the constant `smtpDefaultTimeout`, still 20 seconds, through a small `timeout()` method. No global is mutated, so `-race` stays clean.
**Tests:** `TestSMTPRefusesAServerThatSpeaksOnlyTLS11` (STARTTLS and implicit: `errSMTPTLS`, no login, no AUTH; a TLS 1.2 capped server still works as a control). `TestSMTPDefaultTimeoutAppliesWithoutADeadline` (silent server, `context.Background()`, 300ms default: `context.DeadlineExceeded`, cut near 300ms). `TestSMTPDefaultTimeoutIsTwentySeconds` (constant is 20s, a mailer with no override uses 20s, a mailer from `LoadMailer` has no override).
**Limit to know:** Go clients already default to TLS 1.2 or newer, so deleting the explicit `MinVersion` line would not fail the test. Lowering it to TLS 1.0 does (checked), so the test pins the behaviour against someone loosening it.

### IN-05: SMTP_HOST was not checked at boot

**Files modified:** `internal/mailer/loader.go`, `internal/mailer/loader_host_test.go` (new)
**Commit:** 1ad6ba10
**Applied fix:** A host holding a colon, a slash, any space or any control character is refused, in every env (development too), with "mailer: SMTP_HOST must be a host name such as smtp.gmail.com, with no port, scheme or space". The value is never in the message. Following the instruction, a colon is refused even in an IP literal (so an IPv6 address cannot be used as a host); `127.0.0.1` and `localhost` still load.
**Tests:** ten bad hosts in two envs (port, scheme with and without port, slashes, spaces, tab, bell character, bare and bracketed IPv6) give an error naming SMTP_HOST that repeats nothing; five ordinary hosts still load.

### IN-06: Every space was removed from SMTP_PASSWORD, for any server

**Files modified:** `internal/mailer/loader.go`, `internal/mailer/loader_test.go`, `internal/mailer/loader_password_test.go` (new), `README.md`
**Commit:** 96c6b3cc
**Applied fix:** `normalizePassword` drops the spaces only when `SMTP_HOST` is `smtp.gmail.com` (case blind) and what is left is exactly 16 ASCII letters. Otherwise the password is used as given. The existing trim of leading and trailing whitespace on every setting is unchanged. The first `TestLoadMailer` case (`smtp.example.com`, `"app password"`) now expects the space kept. The README password row says the same.
**Tests:** `TestLoadMailerAppPasswordSpaces`, 11 cases: Gmail with four groups, capitals, uneven groups, host in capitals, already without spaces; Gmail with 15 letters, 17 letters, a digit and a symbol (all keep their spaces); another host with 16 letters in groups, a passphrase, and `smtp.gmail.com.example.net`.
**Decision to confirm:** a Gmail password with an unusual shape keeps its spaces, so a mistyped one fails as a refused sign in, not as a silently reshaped value.

### IN-07: mailcheck printed the "set SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD" hint after every load error

**Files modified:** `internal/mailer/loader.go`, `internal/mailer/loader_required_test.go` (new), `cmd/mailcheck/main.go`, `cmd/mailcheck/hint_test.go` (new)
**Commit:** 8cc9a141
**Applied fix:** The loader marks the errors about SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD (missing, half set group, and the IN-05 bad host) with a small error type; `mailer.IsRequiredSettingError` reports it, and mailcheck prints the hint only then. The review's "when the error names SMTP_HOST" would not work, because the MAIL_FROM error text names SMTP_USERNAME. Error texts are unchanged.
**Tests:** `TestIsRequiredSettingError` (eight loader cases, wrapped or not) and `TestSettingsHintOnlyForTheThreeRequiredSettings` (eight mailcheck cases: hint for nothing set, password only, missing password, missing user name, bad host; no hint for a bad port, a bad sender, a user name that is no address).

### IN-08: Stale comment on the auth write window

**Files modified:** `internal/api/handlers/credentials.go`
**Commit:** 36097bb5
**Applied fix:** The comment now reads "It is longer than the SMTP mailer's 20 second limit (smtpDefaultTimeout in internal/mailer)". Comment only; the package builds.

### IN-09: The README mailcheck step left the app password in the shell and its history

**Files modified:** `README.md`
**Commit:** 3b9e75a8
**Applied fix:** Step 5 of the Gmail owner steps now reads: open a new terminal tab for it, `read -s SMTP_PASSWORD`, `export SMTP_PASSWORD`, export host and user name, run `mailcheck`, `exit`. It says `exit` forgets the password, and that the test copy, `make run` or any server must never be started in that shell. No test pins these lines (checked).

### IN-10: mailcheck linked the whole service package for one constant

**Files modified:** `internal/mailer/mailer.go`, `internal/service/credentials.go`, `internal/service/codelifetime_test.go` (new), `cmd/mailcheck/main.go`, `cmd/mailcheck/imports_test.go` (new)
**Commit:** 079596ea
**Applied fix:** New constant `mailer.CodeLifetime = 10 * time.Minute` in `internal/mailer`. The service's `DefaultCredentialLimits.CodeTTL` is set from it (the service already imports the mailer, so there is no cycle) and mailcheck reads it for the sample email, so mailcheck no longer imports `internal/service`.
**Tests:** `TestCodeLifetimeIsOneNumber` (service package): `CodeTTL` equals `mailer.CodeLifetime`, both are 10 minutes, and the email's minutes value is 10. `TestMailcheckStaysSmall` lists mailcheck's dependencies with `go list -deps` and fails on `internal/service`, `internal/store`, `pgx`, `smtptest` or `testing`; flip checked with a throwaway import of the service (it failed; the file is in no commit).

### IN-11: Two test names and setups promised more than they tested

**Files modified:** `cmd/mailcheck/main_test.go`, `internal/mailer/loader_test.go`
**Commit:** b1d231a0
**Applied fix:** `TestHalfSetSettingsExitNonZero` now sets real half set groups (password only, user name only, port only, sender only, user name and password without a host) with `ENV=development`, and expects a non zero exit that names SMTP_HOST, with empty stdout and no secrets. That `ENV` line now matters: a mailcheck that passed `os.Getenv("ENV")` to the loader would fall back to the log mailer and print a code; checked, the test fails then. The old case (host and user name, no password) is kept as `TestMissingPasswordExitsNonZero`. The no-op `t.Setenv("ENV", "development")` in the loader test is removed. The `ENV` line in `TestMissingSettingsExitNonZeroAndNameThem` was left, since it already serves a purpose.

## Skipped Issues

None.

## Noted, out of scope

- **IN-12 (hosting choice): check that the chosen host allows outgoing SMTP before relying on it.** Left as a note on instruction; nothing was changed for it. The review recalls that Render's free web service no longer opens outgoing SMTP connections, but this review fix made no network call and did not verify it. If it is true, the proposed Render free plan cannot send any sign in email, and README owner step 2 ("run `mailcheck` from the host itself") also needs shell or job access that plan lacks. Before the hosting choice is final: read the host's current outbound SMTP policy, then either pick a paid instance or a host that allows ports 587 and 465, or move to an HTTP based mail API that works with a Gmail account, and make step 2 say what to do when the host has no shell (for example, the first real Create account on the deployed site is the check). The README limits paragraph and the hosting note in the project memory should say what was found.

---

_Fixed: 2026-11-13_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
