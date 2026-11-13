package handlers

// credentials.go puts service.CredentialService on the HTTP surface: the eight
// public POST routes of Create account, the typed code, Sign in, Resend and
// Forgot password (Phase 7.1, plan 07; quick 261113-r5w split the Forgot
// password confirm into two routes, reset/code and reset/password, and the
// eighth, reset/cancel, turns the confirm of step two off). The routes sit outside the access gate
// and behind net/http CrossOriginProtection (see api.NewRouter), and every one
// answers with Cache-Control no-store. No response, header or log line holds an
// email address, a code or a password (D-05, WR-S03).
//
// A wrong sign in is a 400 with the field "credentials", never a 401: a 401
// with the field "auth" means "no signed in session" to the access gate and to
// its tests (research Pitfall 18).
//
// The 10 second write limit. The production server has a WriteTimeout of 10
// seconds, while the Resend client may take a minute. The three handlers that
// send a code email inside the request (SignUp, ResendCode, RequestReset) call
// extendWriteDeadline after the body is decoded and the buckets have agreed,
// the way the cron door in official_refresh.go does after its token check, so a
// slow mail provider is answered and not dropped. The other four handlers
// answer in well under the limit and leave the deadline alone. Plans 08, 09,
// 10, 10b, 14 and 14b follow the same choice for any handler that sends mail.
//
// The same three handlers put the request's sender key (AuthGate.MailSender)
// in the context they pass to the service, for the mail ceiling's per sender
// share of the reset reserve (review CR-S03).

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"time"

	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxCredentialBodyBytes caps every auth request body.
const maxCredentialBodyBytes = 4096

// authWriteWindow is the write deadline a code sending handler gives its one
// answer. It is longer than the mailer's one minute client timeout.
const authWriteWindow = 75 * time.Second

// The sentences that are not in package service. The rate limit sentence is
// shared with internal/ratelimit (tooManyRequestsMessage) and the client side
// auth.js script; all copies must move together (02-14).
const (
	authGenericMessage     = "Something went wrong. Try again."
	authRateLimitedMessage = "Too many requests. Try again in a minute."
)

// authFieldError is the field of the generic answers that name no input.
const authFieldError = "auth"

// SignUpRequest is the body of POST /api/auth/sign-up.
type SignUpRequest struct {
	Email    string `json:"email" example:"visitor@example.com"`
	Password string `json:"password" example:"wide river morning 47"`
}

// SignInRequest is the body of POST /api/auth/sign-in.
type SignInRequest struct {
	Email    string `json:"email" example:"visitor@example.com"`
	Password string `json:"password" example:"wide river morning 47"`
}

// CodeRequest is the body of POST /api/auth/sign-up/code: the six digits the
// person typed, spaces allowed.
type CodeRequest struct {
	Code string `json:"code" example:"482913"`
}

// ResendRequest is the body of POST /api/auth/code/resend. It has no members:
// the browser's own session names the code to send again. The page sends {}.
type ResendRequest struct{}

// ResetRequest is the body of POST /api/auth/reset.
type ResetRequest struct {
	Email string `json:"email" example:"visitor@example.com"`
}

// ResetCodeRequest is the body of POST /api/auth/reset/code, step one of Forgot
// password: the typed code.
type ResetCodeRequest struct {
	Code string `json:"code" example:"482913"`
}

// ResetPasswordRequest is the body of POST /api/auth/reset/password, step two of
// Forgot password: the new password.
type ResetPasswordRequest struct {
	Password string `json:"password" example:"wide river morning 47"`
}

// CodeOKResponse answers a right reset code (step one). It sets no password.
type CodeOKResponse struct {
	CodeOK bool `json:"code_ok" example:"true"`
}

// CodeSentResponse answers every request that may have sent a code, with the
// same bytes for every address.
type CodeSentResponse struct {
	Sent bool `json:"sent" example:"true"`
}

// SignedInResponse answers a good sign in or a confirmed sign up code. Next is
// the page to go to, chosen by the server and never taken from the request.
type SignedInResponse struct {
	Next string `json:"next" example:"/"`
}

// PasswordSetResponse answers a confirmed reset. It signs nobody in.
type PasswordSetResponse struct {
	PasswordSet bool `json:"password_set" example:"true"`
}

// ResetCancelRequest is the body of POST /api/auth/reset/cancel. It has no
// members: the browser's own session names the confirm to turn off. The page
// sends {}.
type ResetCancelRequest struct{}

// ResetCancelledResponse answers POST /api/auth/reset/cancel. It is the same
// bytes whether or not this browser had a confirm to turn off.
type ResetCancelledResponse struct {
	Cancelled bool `json:"cancelled" example:"true"`
}

// writeAuthError writes the shared error envelope with no-store, so every
// refusal of the eight routes, the body ones included, carries it.
func writeAuthError(w http.ResponseWriter, status int, field, message string) {
	w.Header().Set("Cache-Control", "no-store")
	writeFieldError(w, status, field, message)
}

// writeAuthRateLimited writes the one 429 shape with no-store.
func writeAuthRateLimited(w http.ResponseWriter, message string, wait time.Duration) {
	w.Header().Set("Cache-Control", "no-store")
	writeRateLimited(w, "email", message, wait, false, "")
}

// decodeAuthBody applies the rules of the eight routes: application/json only
// (415), a maxCredentialBodyBytes cap (413), unknown fields refused (400) and
// exactly one JSON value (400). Every refusal is the shared envelope with the
// generic sentence: the page's own script always sends a valid body, so no body
// error has copy of its own. It reports whether the request may go on.
func decodeAuthBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeAuthError(w, http.StatusUnsupportedMediaType, "body", authGenericMessage)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err = dec.Decode(dst)
	if err == nil {
		// Exactly one value: anything after it, other than space, is refused.
		if _, next := dec.Token(); !errors.Is(next, io.EOF) {
			err = errors.New("trailing data")
		}
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeAuthError(w, http.StatusRequestEntityTooLarge, "body", authGenericMessage)
			return false
		}
		writeAuthError(w, http.StatusBadRequest, "body", authGenericMessage)
		return false
	}
	return true
}

// extendWriteDeadline gives this one response a write deadline that outlives a
// slow mail provider (see the note at the top of this file). A writer that
// cannot take a deadline, such as a test recorder, is left alone. The deadline
// is set only on the sending handlers, after the body and the buckets passed.
func extendWriteDeadline(w http.ResponseWriter) {
	err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(authWriteWindow))
	if err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Printf("handlers: auth: write deadline class=network")
	}
}

// signedInNext is the page a signed in person is sent to: the Low data view
// when this device remembers that choice, the main page otherwise. It reads
// nothing from the request body or the query (open redirect).
func signedInNext(r *http.Request) string {
	if lowDataChosen(r) {
		return LowDataPath
	}
	return "/"
}

// writeCredentialError maps a CredentialService error to the one answer the
// page reads. The sentences come from package service, so the words exist once.
func writeCredentialError(w http.ResponseWriter, handler string, err error) {
	var ve service.ValidationError
	var locked service.SignInLockedError
	switch {
	case errors.As(err, &ve):
		writeAuthError(w, http.StatusBadRequest, ve.Field, ve.Message)
	case errors.Is(err, service.ErrWrongCredentials):
		// A 400 on the field credentials, never a 401 (research Pitfall 18).
		writeAuthError(w, http.StatusBadRequest, "credentials", service.WrongCredentialsMessage)
	case errors.Is(err, service.ErrWrongCode):
		writeAuthError(w, http.StatusBadRequest, "code", service.WrongCodeMessage)
	case errors.Is(err, service.ErrRateLimited):
		// This sentence is duplicated in internal/ratelimit's
		// tooManyRequestsMessage and in the client-side auth.js script; all
		// copies must move together (02-14).
		writeAuthRateLimited(w, authRateLimitedMessage, time.Minute)
	case errors.Is(err, service.ErrTooManyCodes):
		writeAuthRateLimited(w, service.TooManyCodesMessage, time.Hour)
	case errors.As(err, &locked):
		writeAuthRateLimited(w, service.SignInLockedMessage(locked.Wait), locked.Wait)
	case errors.Is(err, service.ErrMailUnavailable), errors.Is(err, service.ErrBusy):
		writeAuthError(w, http.StatusServiceUnavailable, "email", authGenericMessage)
	default:
		// The service scrubs its errors of addresses, codes and passwords.
		log.Printf("handlers: %s: %v", handler, err)
		writeAuthError(w, http.StatusInternalServerError, authFieldError, authGenericMessage)
	}
}

// authReady runs the start every handler shares: no-store first, then the 503
// for a router built without a CredentialService (a Deps literal that omits it
// still serves every other route), then the session id the middleware resolved.
func authReady(w http.ResponseWriter, r *http.Request, creds *service.CredentialService) (sessionID string, ok bool) {
	w.Header().Set("Cache-Control", "no-store")
	if creds == nil {
		writeAuthError(w, http.StatusServiceUnavailable, authFieldError, authGenericMessage)
		return "", false
	}
	id, found := session.FromContext(r.Context())
	if !found {
		log.Printf("handlers: auth: no session in request context")
		writeAuthError(w, http.StatusInternalServerError, authFieldError, authGenericMessage)
		return "", false
	}
	return id, true
}

// isValidation reports whether err is a ValidationError, the one refusal that
// proves nothing was sent or counted.
func isValidation(err error) bool {
	var ve service.ValidationError
	return errors.As(err, &ve)
}

// SignUp handles POST /api/auth/sign-up (Create account): the email and the
// password go in, one code email goes out, and the answer is the same bytes for
// every address, so the route never says whether an account exists. The send
// token is given back when the service refused the input and sent nothing.
//
// @Summary      Create an account with an email and a password
// @Description  Checks the email and the password, then emails a 6 digit code to type on the next
// @Description  screen. The answer is the same for every address, so it never says whether an
// @Description  account already exists. The password is held only as a hash until the code is
// @Description  confirmed. The response never carries a session identifier.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      SignUpRequest  true  "Email and password"
// @Success      200   {object}  CodeSentResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/sign-up [post]
func SignUp(creds *service.CredentialService, gate *AuthGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req SignUpRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		send := gate.ReserveCodeSend(r)
		if !send.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, send.RetryAfter)
			return
		}
		extendWriteDeadline(w)
		if err := creds.SignUp(withMailSender(r, gate), sessionID, req.Email, req.Password); err != nil {
			if isValidation(err) {
				send.Cancel()
			}
			writeCredentialError(w, "SignUp", err)
			return
		}
		writeJSON(w, http.StatusOK, CodeSentResponse{Sent: true})
	}
}

// ConfirmSignUp handles POST /api/auth/sign-up/code: the typed code. A right
// code creates the account and signs the browser in on a fresh session id; the
// cookie for that id is set before any body is written.
//
// @Summary      Confirm the code of a new account
// @Description  Checks the 6 digit code typed by the person. A right code creates the account, sets
// @Description  the password typed at Create account and signs this browser in. The response names
// @Description  the page to go to next and never carries a session identifier.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      CodeRequest  true  "The typed code"
// @Success      200   {object}  SignedInResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/sign-up/code [post]
func ConfirmSignUp(creds *service.CredentialService, gate *AuthGate, mgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req CodeRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		try := gate.ReserveCodeTry(r)
		if !try.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, try.RetryAfter)
			return
		}
		result, err := creds.ConfirmSignUp(r.Context(), sessionID, req.Code, mgr.NewID)
		if err != nil {
			writeCredentialError(w, "ConfirmSignUp", err)
			return
		}
		try.Cancel()
		mgr.SetCookie(w, result.SessionID)
		writeJSON(w, http.StatusOK, SignedInResponse{Next: signedInNext(r)})
	}
}

// ResendCode handles POST /api/auth/code/resend: send the code of this
// browser's newest code screen again. The body is the empty object.
//
// @Summary      Send the code again
// @Description  Sends a new code for the code screen this browser is showing, under the same
// @Description  per address limits. The body is an empty object. The answer is the same for every
// @Description  address.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ResendRequest  true  "Empty object"
// @Success      200   {object}  CodeSentResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/code/resend [post]
func ResendCode(creds *service.CredentialService, gate *AuthGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req ResendRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		send := gate.ReserveCodeSend(r)
		if !send.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, send.RetryAfter)
			return
		}
		extendWriteDeadline(w)
		if err := creds.ResendCode(withMailSender(r, gate), sessionID); err != nil {
			if isValidation(err) {
				send.Cancel()
			}
			writeCredentialError(w, "ResendCode", err)
			return
		}
		writeJSON(w, http.StatusOK, CodeSentResponse{Sent: true})
	}
}

// SignIn handles POST /api/auth/sign-in. A wrong email or password is one
// answer: 400 on the field credentials with WrongCredentialsMessage. A
// good sign in refunds the failure token, sets the cookie of the fresh session
// id and answers the page to go to.
//
// @Summary      Sign in with an email and a password
// @Description  Checks the email and the password. A wrong email, a wrong password and an account
// @Description  without a password all get the same answer. A good sign in puts this browser on a
// @Description  new session and names the page to go to next. The response never carries a session
// @Description  identifier.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      SignInRequest  true  "Email and password"
// @Success      200   {object}  SignedInResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/sign-in [post]
func SignIn(creds *service.CredentialService, gate *AuthGate, mgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req SignInRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		attempt := gate.ReserveSignIn(r)
		if !attempt.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, attempt.RetryAfter)
			return
		}
		result, err := creds.SignIn(r.Context(), sessionID, req.Email, req.Password, mgr.NewID)
		if err != nil {
			writeCredentialError(w, "SignIn", err)
			return
		}
		attempt.Cancel()
		mgr.SetCookie(w, result.SessionID)
		writeJSON(w, http.StatusOK, SignedInResponse{Next: signedInNext(r)})
	}
}

// RequestReset handles POST /api/auth/reset (Forgot password): one email goes
// out for every address, a code for an account and a "there is no account"
// note for an unknown address, and the answer is the same bytes either way.
//
// @Summary      Ask for a password reset code
// @Description  Emails a 6 digit code to an address that has an account, and a short note to an
// @Description  address that has none. The answer is the same for every address, so it never says
// @Description  whether an account exists.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ResetRequest  true  "Email"
// @Success      200   {object}  CodeSentResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/reset [post]
func RequestReset(creds *service.CredentialService, gate *AuthGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req ResetRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		send := gate.ReserveCodeSend(r)
		if !send.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, send.RetryAfter)
			return
		}
		extendWriteDeadline(w)
		if err := creds.RequestReset(withMailSender(r, gate), sessionID, req.Email); err != nil {
			if isValidation(err) {
				send.Cancel()
			}
			writeCredentialError(w, "RequestReset", err)
			return
		}
		writeJSON(w, http.StatusOK, CodeSentResponse{Sent: true})
	}
}

// ConfirmResetCode handles POST /api/auth/reset/code, step one of Forgot
// password: the typed code and nothing else. A right code is remembered for this
// browser for 5 minutes and opens step two, and the browser moves to a freshly
// drawn session id (the new cookie rides on the answer); it sets no password and
// signs nobody in. A wrong code is counted against the same budget of five tries
// as ever, sets no cookie, and the answer is the same for an address with an
// account and one without.
//
// @Summary      Confirm the reset code
// @Description  Checks the 6 digit code typed on the Forgot password screen. A right code is
// @Description  remembered for this browser for 5 minutes, so the next call can set the new
// @Description  password, and this browser is moved to a new session in the same answer. It sets
// @Description  no password and signs nobody in. A wrong, expired or replaced code gets the same
// @Description  answer, whether or not the address has an account, and changes no cookie.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ResetCodeRequest  true  "The typed code"
// @Success      200   {object}  CodeOKResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/reset/code [post]
func ConfirmResetCode(creds *service.CredentialService, gate *AuthGate, mgr *session.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req ResetCodeRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		try := gate.ReserveCodeTry(r)
		if !try.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, try.RetryAfter)
			return
		}
		fresh, err := creds.CheckResetCode(r.Context(), sessionID, req.Code, mgr.NewID)
		if err != nil {
			writeCredentialError(w, "ConfirmResetCode", err)
			return
		}
		try.Cancel()
		// The confirmed code now belongs to a freshly drawn session id, which only
		// this answer carries (review fix WR-02); a wrong code sets no cookie.
		mgr.SetCookie(w, fresh)
		writeJSON(w, http.StatusOK, CodeOKResponse{CodeOK: true})
	}
}

// SetResetPassword handles POST /api/auth/reset/password, step two of Forgot
// password: the new password, for a browser whose code was confirmed by step
// one. It ends every session of the account and signs nobody in; the person
// signs in with the new password. It takes a token from the same bucket as the
// code tries, because a request that reaches the hasher is costly; the token
// goes back on success and when the new password itself was refused, which
// counts nothing and keeps the confirm.
//
// @Summary      Set a new password after the reset code was confirmed
// @Description  Sets the new password for a browser whose reset code was confirmed less than 5 minutes
// @Description  ago. A browser with no confirmed code, a confirm that ran out, or a code replaced by
// @Description  a newer one gets the wrong code answer. A good call ends every session of the
// @Description  account and does not sign this browser in.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ResetPasswordRequest  true  "The new password"
// @Success      200   {object}  PasswordSetResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      429   {object}  RateLimitResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/reset/password [post]
func SetResetPassword(creds *service.CredentialService, gate *AuthGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req ResetPasswordRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		try := gate.ReserveCodeTry(r)
		if !try.OK {
			writeAuthRateLimited(w, authRateLimitedMessage, try.RetryAfter)
			return
		}
		if err := creds.ConfirmReset(r.Context(), sessionID, req.Password); err != nil {
			// A refused new password is checked before anything is counted or
			// spent, so its token goes back.
			if isValidation(err) {
				try.Cancel()
			}
			writeCredentialError(w, "SetResetPassword", err)
			return
		}
		try.Cancel()
		writeJSON(w, http.StatusOK, PasswordSetResponse{PasswordSet: true})
	}
}

// CancelReset handles POST /api/auth/reset/cancel: Change email on step two of
// Forgot password turns this browser's confirm off on the server, so a reload
// afterwards draws the code screen and never step two (review fix WR-01). It
// sits behind the same cross site guard and strict body as its siblings, and it
// takes no bucket token: it sends nothing, hashes nothing and spends nothing,
// and the one thing it changes is the confirm of the caller's own session. The
// answer is the same bytes whether or not there was a confirm to turn off, so
// it says nothing about the browser's code or about any address.
//
// @Summary      Turn off the confirmed reset code of this browser
// @Description  Called when the person leaves the new password step with Change email. Clears the
// @Description  confirm of this browser's newest unused reset code, so the code has to be typed
// @Description  again before a new password is accepted. The body is an empty object. The answer is
// @Description  the same whether or not there was a confirm to clear.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ResetCancelRequest  true  "Empty object"
// @Success      200   {object}  ResetCancelledResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse
// @Failure      415   {object}  ErrorResponse
// @Failure      503   {object}  ErrorResponse
// @Router       /auth/reset/cancel [post]
func CancelReset(creds *service.CredentialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := authReady(w, r, creds)
		if !ok {
			return
		}
		var req ResetCancelRequest
		if !decodeAuthBody(w, r, &req) {
			return
		}
		if err := creds.CancelResetConfirm(r.Context(), sessionID); err != nil {
			writeCredentialError(w, "CancelReset", err)
			return
		}
		writeJSON(w, http.StatusOK, ResetCancelledResponse{Cancelled: true})
	}
}
