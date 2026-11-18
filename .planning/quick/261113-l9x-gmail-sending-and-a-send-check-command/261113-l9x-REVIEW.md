---
phase: quick-261113-l9x
reviewed: 2026-11-13T00:00:00Z
depth: deep
files_reviewed: 18
files_reviewed_list:
  - internal/mailer/smtp.go
  - internal/mailer/smtp_test.go
  - internal/mailer/smtptest/smtptest.go
  - internal/mailer/loader.go
  - internal/mailer/loader_test.go
  - internal/mailer/mailer.go
  - internal/mailer/logmailer.go
  - internal/mailer/ceiling.go
  - cmd/mailcheck/main.go
  - cmd/mailcheck/main_test.go
  - cmd/server/main.go
  - internal/api/handlers/credentials.go
  - go.mod
  - go.sum
  - README.md
  - docs/TRUST-MODEL.md
  - web/templates/legal_body.html.tmpl
  - web/review71_templates_fix_contract_test.go
findings:
  critical: 0
  warning: 1
  info: 11
  total: 12
  necessity: 0
status: issues_found
---

# Quick 261113-l9x: Code Review Report

**Reviewed:** 2026-11-13
**Depth:** deep (mailer, loader, fake server, send check command, the service and handler that call the mailer, docs and the Policy line)
**Files Reviewed:** 18 source and doc files from the diff `e754b884..e0480e6d` (`internal/mailer/resend.go` and `resend_test.go` were deleted and are checked only for leftovers)
**Status:** issues_found

## Summary

The security rules of the plan hold. I found no path where the app password, the code, the user name or a recipient address can reach the wire unencrypted, an error, a log line or the mailcheck output today.

- **TLS before AUTH, both paths.** On port 465 the TLS handshake (`smtp.go:249-255`) runs before `smtp.NewClient` reads a single byte. On any other port the client needs STARTTLS in the EHLO answer (`smtp.go:266-272`). If EHLO fails, Go falls back to HELO, which clears the extension list, so the send ends with `errSMTPNoTLS` and never goes plain. Stripping STARTTLS, refusing it, or failing the handshake all fail closed. A second check (`smtp.go:276-278`) refuses to go on unless the connection really is TLS. That check matters, because Go's `smtp.PlainAuth` would send a password in plain text to a server named `127.0.0.1` or `localhost`. `TestSMTPNoSTARTTLSGetsNoAuth` uses exactly such a server, so it proves our own refusal and not Go's.
- **Certificate and name.** `ServerName` is the configured host, `MinVersion` is TLS 1.2, and `RootCAs` is nil (the system roots) on every production path. `InsecureSkipVerify` appears nowhere in the repo. The STARTTLS upgrade starts a fresh reader, so any plain text injected after the `220` is thrown away.
- **The exported `NewSMTPMailerWithRootCAs`.** Only `cmd/mailcheck/main_test.go` calls it. The only production constructors are in `loader.go:56` and `loader.go:92`. Even a bad pool cannot switch the checks off: chain, name and version are still verified. An empty pool fails closed. Only a pool that holds an attacker's CA would be dangerous, and no code builds one (see IN-03).
- **`smtptest` and `testing`.** `go list -deps ./cmd/server ./cmd/mailcheck` lists neither `smtptest` nor `testing`, so the fake server cannot end up in a shipped binary.
- **Errors.** `smtpFailure` keeps only a sentinel and the reply code, and drops the server's words. The loader errors name a setting and never its value. mailcheck prints only those strings. `TestSMTPFailureKinds` and `TestFailureShowsOnlyTheKind` feed in server words that repeat the address and the user name, and pin that both are dropped.
- **Header injection and MIME.** The recipient must be exactly one bare address: `ParseAddress(to).Address == to` with no display name, plus a check for CR, LF and NUL. The sender is parsed at load time and again at send time. The subject is a constant and is Q encoded when it is not ASCII. I probed all four kinds in a scratch copy of the package. None has a bare CR or a bare LF, and the longest line is 106 bytes, the multipart `Content-Type` header with its boundary, far under 998. The boundary comes from crypto/rand. Both parts are UTF-8 quoted-printable. Date is RFC 1123Z. Message-ID is 128 random bits at the From domain.
- **Time limits.** The whole exchange is bounded by the context deadline, or by 20 seconds when there is none. The deadline is set on the raw socket and also enforced by a `context.AfterFunc` that closes it. Every exit closes the connection, through the deferred `raw.Close` and `client.Close`. The handler's 75 second write window (`credentials.go:45`) is longer than the 20 seconds.
- **The loader.** A production server cannot boot with the log mailer: `NewLogMailer` is returned only when `env == "development"` (`loader.go:50-56`). Every other value, including an empty one and a value with different capital letters, gets an error. A half set group outside development is an error, and in development it is a warning plus the log mailer, as planned.
- **Resend.** No Resend provider code is left. `git grep` finds no `RESEND_`, `resend-go`, `resend.dev` or provider wording outside `.planning`, and `go mod tidy -diff` prints nothing. The Resend code button, its routes, its queries and its messages are untouched. The only handler change is one comment line in `credentials.go`, and every remaining hit for the word means the button or the cooldown.

`go vet` and `gofmt -l` are clean for the mailer and mailcheck packages. `go test -race -count=1 ./internal/mailer/... ./cmd/mailcheck/` and `TestPolicyNamesHomeSquareScreeningAndMail` pass. No test or probe in this review touched the network or a database.

The one warning is a latent leak. The real app password prints whenever anyone formats the mailer or its config with `%v`. Nothing does that today, but nothing stops it either. The info items are about wording: a few failures get a label that would send the owner to the wrong fix, two security claims have no test, there is one stale comment, and there are some README and hosting notes.

Necessity was assessed using `necessity-review.md` and found nothing. Both of the redundant looking checks (the `TLSConnectionState` check and the second line break check in `buildMessage`) are deliberate defence in depth, and the From check guards a value from the environment, a trust boundary.

## Warnings

### WR-01: The app password prints under %v and %+v of the mailer or its config

**Files:**
- `internal/mailer/smtp.go:27-35` (`SMTPConfig`, exported, holds `Password`)
- `internal/mailer/smtp.go:78-86` (`smtpMailer` embeds the config)

**Issue:** Neither type has a `String`, `GoString` or `Format` method. A scratch probe that ran `LoadMailer("production")` with made up settings and formatted the result printed:

```
%v:  &{{smtp.example.com 587 u@example.com PROBEPASSWORD "Pinalert" <u@example.com>} <nil> false}
%+v: &{cfg:{Host:smtp.example.com Port:587 Username:u@example.com Password:PROBEPASSWORD ...} ...}
```

No line in the server does this today. I grepped `cmd/server` and `internal/api` for `%v` and `%+v` applied to mailer or config values. Still, one debug line such as `log.Printf("mailer: %+v", mail)` in `cmd/server/main.go` (before the ceiling wraps it), or any print of an `SMTPConfig` value, would write the real Gmail app password into the host's log. The plan's must have reads "no secret ... ever reaches a log line", and the owner is about to put a live password behind this type. The loader tests already print `m` with `%v` on their failure paths (`loader_test.go:111`, `loader_test.go:155`).

**Fix:** Redact at the type, and pin it with a test:

```go
// String and GoString keep the password out of every fmt verb.
func (c SMTPConfig) String() string {
	return fmt.Sprintf("SMTPConfig{Host:%s Port:%d Username:[set] Password:[redacted] From:[set]}", c.Host, c.Port)
}
func (c SMTPConfig) GoString() string { return c.String() }

func (m *smtpMailer) String() string   { return "smtpMailer{" + m.cfg.String() + "}" }
func (m *smtpMailer) GoString() string { return m.String() }
```

Add a test that formats a mailer from `LoadMailer` with `%v`, `%+v`, `%#v` and `%s`, and asserts that the password and the user name are absent. A second probe showed the limits of the leak: a struct that holds the mailer in an interface field, such as the ceiling or the router deps, prints only a pointer address, so the exposure is a direct print of the `*smtpMailer` or of an `SMTPConfig` value. fmt never calls a `String` method on an unexported field, so a struct that holds an `SMTPConfig` by value in an unexported field would still print the password even after this fix. Keep the config behind a pointer or an interface wherever it is stored.

## Info

### IN-01: A refused STARTTLS is labelled by its reply code, not as a TLS failure

**File:** `internal/mailer/smtp.go:270-272`, `internal/mailer/smtp.go:321-331`
**Issue:** `smtpFailure` checks for a reply code before it looks at the stage. In a scratch probe, a server that advertised STARTTLS and answered it with `454` gave "the mail server asked to try again later (reply 454)". A `5xx` answer gives "the mail server refused the message", which suggests the message was the problem, not the encryption. This is safe: the probe server saw no AUTH and no command at all after the refusal. But the log line points the owner the wrong way. No test pins the refused STARTTLS path, so the fact that nothing follows it is unguarded.
**Fix:** At `stageTLS`, return `errSMTPTLS` with the reply code: `fmt.Errorf("%w (reply %d)", errSMTPTLS, reply.Code)`. Add a smtptest option such as `STARTTLSReply`, plus a test that a refused STARTTLS records no AUTH and no MAIL.

### IN-02: "The mail server refused the sign in" also covers a missing PLAIN and a dropped connection

**File:** `internal/mailer/smtp.go:280-282`, `internal/mailer/smtp.go:332-336`
**Issue:** Two failures that are not a refused password get `errSMTPAuth`: a server that offers no AUTH PLAIN after TLS, and a connection that drops in the middle of AUTH (an `io.EOF` from the read, not a `textproto.Error`). The mailcheck test and the README both tell the owner that this wording means the sign in was refused, so the owner would revoke and recreate a good app password.
**Fix:** Add a separate sentinel for the first, such as "the mail server offers no password sign in". For the second, keep `errSMTPAuth` only when the error is a `*textproto.Error`, and return `errSMTPNetwork` otherwise.

### IN-03: The two test only exports are kept out of production by convention only

**File:** `internal/mailer/smtp.go:103-111`, `internal/mailer/smtptest/smtptest.go:5`, `internal/mailer/smtptest/smtptest.go:21`
**Issue:** The doc comments say that nothing in the server's startup calls `NewSMTPMailerWithRootCAs` and that nothing outside a `_test.go` file imports `smtptest`. Both are true today (`git grep` and `go list -deps`), but no test enforces either. A later change could load a trust pool from an environment variable, through a "custom CA" setting, and hand it to the exported constructor. That is the only way the exported hook could weaken the TLS check.
**Fix:** Add one guard test, for example in `cmd/server`, that runs `go list -deps ./cmd/server ./cmd/mailcheck` and fails if `testing` or `pinalert/internal/mailer/smtptest` appears. Have the same test grep the non test `.go` files for `NewSMTPMailerWithRootCAs` and fail on any hit outside `internal/mailer/smtp.go`. Another option is to drop the export: give `run` in mailcheck a loader seam that returns any `mailer.Mailer`, and build the TLS mailer for the test through a small exported test helper in `smtptest` that takes the pool.

### IN-04: Two of the stated guarantees have no test: TLS 1.2 or newer, and the 20 second limit with no deadline

**File:** `internal/mailer/smtp.go:229`, `internal/mailer/smtp.go:220-224`, `internal/mailer/smtptest/smtptest.go:89`, `internal/mailer/smtptest/smtptest.go:235`
**Issue:** The fake server sets `MinVersion: tls.VersionTLS12` itself and never offers an older version, so deleting the client's `MinVersion` would pass every test. The hang tests always pass a context with its own deadline. If the `WithTimeout` branch were deleted, `SetDeadline(time.Time{})` would mean no deadline at all, and mailcheck, which uses `context.Background()`, would hang for ever on a silent server. No test would notice.
**Fix:** Add a smtptest option `MaxVersion` and a test where the server caps at TLS 1.1 and the client refuses with `errSMTPTLS` before any AUTH. Make `smtpDefaultTimeout` a package variable that the test can shorten, and run the `Hang` server with `context.Background()`.

### IN-05: SMTP_HOST is not checked at boot

**File:** `internal/mailer/loader.go:39`
**Issue:** Values such as `smtp.gmail.com:587`, `smtps://smtp.gmail.com` or a host with a space inside pass the loader. `net.JoinHostPort` then produces an address that cannot be dialed, and every send fails later as "could not reach the mail server". This is the same class of mistake the loader refuses at boot for SMTP_PORT and MAIL_FROM.
**Fix:** Refuse at boot any host that holds `:`, `/`, a space or a control character, unless it parses as an IP address, with "mailer: SMTP_HOST must be a host name such as smtp.gmail.com, with no port or scheme".

### IN-06: Every space is removed from SMTP_PASSWORD, for any server

**File:** `internal/mailer/loader.go:42-44`
**Issue:** This is right for a Google app password, which is 16 letters shown in groups. But SMTP_HOST is generic, and the password of any other server that holds a space is silently changed. The result is an auth refusal that matches nothing the owner typed.
**Fix:** Strip the spaces only when the stripped value is 16 ASCII letters, or say in the README table that the spaces are dropped for every server.

### IN-07: mailcheck prints the "set SMTP_HOST, SMTP_USERNAME and SMTP_PASSWORD" hint after every load error

**File:** `cmd/mailcheck/main.go:84-85`
**Issue:** After a bad SMTP_PORT or a bad MAIL_FROM, the loader's own error is right, but the second line still tells the owner to set the three settings, which are already set.
**Fix:** Print only the loader's error, or print the hint only when the error names SMTP_HOST.

### IN-08: Stale comment on the auth write window

**File:** `internal/api/handlers/credentials.go:43-44`
**Issue:** The comment reads "It is longer than the mailer's one minute client timeout". That was the Resend client. The limit is now 20 seconds (`smtp.go:39`), and the file's head comment was updated to say so, so the two comments now disagree. The 75 second value is still safe.
**Fix:** "It is longer than the SMTP mailer's 20 second limit (smtpDefaultTimeout)".

### IN-09: The README's mailcheck step leaves the app password in the shell, and in its history

**File:** `README.md:148-150`
**Issue:** "set the four settings in your shell" invites `export SMTP_PASSWORD=...`. That line stays in `~/.zsh_history`. Every later command in that shell then sees the real Gmail settings too, including `make run` or a test copy server, which would send real mail from the Pinalert account to the made up `example.com` people, spend the day's quota and hurt the new account's standing. The memory's UAT recipe uses `env -i`, but the README does not warn about this.
**Fix:** Show a form that keeps the password out of the history and out of the shell, for example:

```sh
read -rs SMTP_PASSWORD   # paste, nothing is shown
SMTP_HOST=smtp.gmail.com SMTP_USERNAME="<the sending Gmail address>" SMTP_PASSWORD="$SMTP_PASSWORD" \
  go run ./cmd/mailcheck -to your.own@address.example
unset SMTP_PASSWORD
```

### IN-10: mailcheck links the whole service package for one constant

**File:** `cmd/mailcheck/main.go:32`, `cmd/mailcheck/main.go:98`
**Issue:** Importing `pinalert/internal/service` for `DefaultCredentialLimits.CodeTTL` pulls pgx, sqlc, push, moderation, geocode and areas into a command that "touches no database". I checked: none of them has an `init` that reads the environment or the network, so there is no runtime effect today. But any init added there later runs inside mailcheck too.
**Fix:** Acceptable as is, because it keeps the lifetime in one place. If the import ever matters, move `CodeTTL` into a small dependency free package that both `service` and `mailcheck` import.

### IN-11: Two test names and setups promise more than they test

**File:** `cmd/mailcheck/main_test.go:120-132`, `internal/mailer/loader_test.go:72`
**Issue:** `TestHalfSetSettingsExitNonZero` sets SMTP_HOST and SMTP_USERNAME but no password. That is the "host set, password missing" branch, not the half set group the loader comment means (some setting set, SMTP_HOST not). The half set group is covered at the loader level only. `loader_test.go:72` sets `ENV=development`, but `LoadMailer` takes the env as an argument and never reads `ENV`, so that line does nothing and suggests a dependency that does not exist.
**Fix:** Rename the mailcheck test to `TestMissingPasswordExitsNonZero` and add a case with only SMTP_PASSWORD set. Delete the `t.Setenv("ENV", ...)` line from the loader test.

### IN-12: Check that the chosen host allows outgoing SMTP before relying on it (unverified here)

**File:** `README.md:155-157`, `README.md:167-168`
**Issue:** The README rightly says some free hosts block ports 465 and 587. The current hosting proposal in the project notes is Render's free web service. I recall that Render announced in 2025 that free web services can no longer open outgoing SMTP connections, but this review made no network call, so I have not verified it. If it is true, the proposed host cannot send any sign in email, and step 2 of the pre-launch list ("run `mailcheck` from the host itself, or from a one-off job") also needs shell or job access, which Render's free plan does not offer.
**Fix:** Before the hosting choice is final, check the host's current outbound SMTP policy on its own docs. If the free plan blocks SMTP, note it in the README's limits paragraph and in the hosting memory. Then either pick a paid instance or a host that allows ports 587 and 465, or use an HTTP based mail API that works with a Gmail account. Make step 2 say what to do when the host has no shell, for example "the first real Create account on the deployed site is the check".

---

_Reviewed: 2026-11-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
