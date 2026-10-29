package handlers

// authlimits.go holds the in memory buckets of the code and password routes
// (Phase 7.1, plan 07, D-02). Every bucket is a ratelimit.PerKey. The keys are:
//
//   - the device: the signed session id the browser carries, so one browser
//     cannot guess passwords or codes, or ask for emails, without limit;
//   - the network: the client IP, only when CLIENT_IP_HEADER names a header the
//     host overwrites, and then only as the IPv4 address or the IPv6 /64.
//
// The network bucket is off without CLIENT_IP_HEADER on purpose. Without the
// header the only address Go can see is the TCP peer, and on a host behind a
// proxy or a carrier gateway that address is shared by many people, so a bucket
// on it would lock out everyone because of one abuser (research Pitfall 9,
// CGNAT, PROJECT.md). The per address throttle that does not depend on the
// network lives in the database instead (plan 01, auth_throttles), and the
// hashing cost bounds the rest.
//
// No limit number is typed here: the numbers live in the constants block of
// cmd/server/main.go and in its mirror api.AuthLimitsDefault.

import (
	"context"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"pinalert/internal/mailer"
	"pinalert/internal/ratelimit"
	"pinalert/internal/session"
)

// AuthLimits holds the allowances of the auth routes. The api package aliases
// this type and owns the defaults.
type AuthLimits struct {
	// SignInDevice is the wrong sign in allowance per browser.
	SignInDevice BucketLimit
	// SignInIP is the loose network allowance for sign in, used only with
	// CLIENT_IP_HEADER.
	SignInIP BucketLimit
	// CodeSendDevice is the allowance of requests that send a code email per
	// browser (Create account, Resend, Forgot password).
	CodeSendDevice BucketLimit
	// CodeSendIP is the loose network allowance for the same, used only with
	// CLIENT_IP_HEADER.
	CodeSendIP BucketLimit
	// CodeSendNoCookie is one small bucket shared by every code send that
	// arrived without a valid session cookie: none at all, a malformed one or a
	// wrongly signed one, so the session middleware had to mint a fresh id
	// (review WR-S01). A browser on the sign in page always has the cookie GET
	// /login set, so only a script lands here, and a junk cookie cannot buy it a
	// fresh device bucket each time. A script that first GETs /login for a valid
	// fresh cookie is bounded by the network bucket and the mail pacing instead.
	CodeSendNoCookie BucketLimit
	// SignInNoCookie is the same shared bucket for sign in tries (review
	// IN-S05), so a script without a valid cookie cannot keep the password hash
	// slots busy from a fresh device bucket on every request.
	SignInNoCookie BucketLimit
	// CodeTryDevice is the allowance of typed code tries per browser.
	CodeTryDevice BucketLimit
	// UsernameCheck and UsernameSave are per account allowances for the
	// username routes of plan 10.
	UsernameCheck BucketLimit
	UsernameSave  BucketLimit
}

// cookielessKey is the one key of the shared buckets for requests that carry
// no valid session cookie.
const cookielessKey = "no-cookie"

// AuthGate holds the auth route buckets.
type AuthGate struct {
	signInDevice     *ratelimit.PerKey
	signInIP         *ratelimit.PerKey
	codeSendDevice   *ratelimit.PerKey
	codeSendIP       *ratelimit.PerKey
	codeSendNoCookie *ratelimit.PerKey
	signInNoCookie   *ratelimit.PerKey
	codeTryDevice    *ratelimit.PerKey
	usernameCheck    *ratelimit.PerKey
	usernameSave     *ratelimit.PerKey
	backstopOn       bool
}

// NewAuthGate builds the gate from fully resolved limits. A nil clock means the
// wall clock. The network buckets are on only when clientIPHeader is non empty.
func NewAuthGate(l AuthLimits, clock func() time.Time, clientIPHeader string) *AuthGate {
	if clock == nil {
		clock = time.Now
	}
	return &AuthGate{
		signInDevice:     ratelimit.NewPerKey(bucketConfig(l.SignInDevice, clock)),
		signInIP:         ratelimit.NewPerKey(bucketConfig(l.SignInIP, clock)),
		codeSendDevice:   ratelimit.NewPerKey(bucketConfig(l.CodeSendDevice, clock)),
		codeSendIP:       ratelimit.NewPerKey(bucketConfig(l.CodeSendIP, clock)),
		codeSendNoCookie: ratelimit.NewPerKey(bucketConfig(l.CodeSendNoCookie, clock)),
		signInNoCookie:   ratelimit.NewPerKey(bucketConfig(l.SignInNoCookie, clock)),
		codeTryDevice:    ratelimit.NewPerKey(bucketConfig(l.CodeTryDevice, clock)),
		usernameCheck:    ratelimit.NewPerKey(bucketConfig(l.UsernameCheck, clock)),
		usernameSave:     ratelimit.NewPerKey(bucketConfig(l.UsernameSave, clock)),
		backstopOn:       strings.TrimSpace(clientIPHeader) != "",
	}
}

// admitted is the decision of a reservation that took nothing.
func admitted() ratelimit.Decision {
	return ratelimit.Decision{OK: true, Cancel: func() {}}
}

// deviceKey is the bucket key of one browser: its signed session id. A request
// with no session in its context (only a test without the session middleware)
// shares the key "sess:".
func deviceKey(sessionID string) string {
	return "sess:" + sessionID
}

// ipBucketKey is the network bucket key of a client address: the address itself
// for IPv4 (an IPv4 mapped IPv6 address counts as the IPv4 address) and the /64
// prefix for IPv6, because one home or phone connection holds a whole /64 and
// a person can pick any address inside it. An empty or unparsable address gives
// "", which means the bucket is skipped.
func ipBucketKey(ip string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return ""
	}
	return prefix.String()
}

// arrivedWithoutValidCookie reports whether the request carried no valid
// session cookie: the session middleware minted a fresh id because the cookie
// was absent, malformed or wrongly signed (review WR-S01). A request that never
// went through the middleware (only a test) falls back to cookie presence.
func arrivedWithoutValidCookie(r *http.Request) bool {
	if minted, known := session.Minted(r.Context()); known {
		return minted
	}
	_, err := r.Cookie(session.CookieName)
	return err != nil
}

// sessionIDOf is the session id of the request, "" when the context has none.
func sessionIDOf(r *http.Request) string {
	id, _ := session.FromContext(r.Context())
	return id
}

// reserveIP takes one token of a network bucket for the request's client IP. It
// admits the request without spending anything when the network buckets are off
// or when the router found no valid client address. It never falls back to the
// connection's peer address (reserveIPBackstop has the same rule).
func (g *AuthGate) reserveIP(r *http.Request, bucket *ratelimit.PerKey) ratelimit.Decision {
	if !g.backstopOn {
		return admitted()
	}
	key := ipBucketKey(middleware.GetClientIP(r.Context()))
	if key == "" {
		return admitted()
	}
	return bucket.Reserve(key)
}

// refuse cancels every reservation already taken and returns the refusing
// decision, whose own Cancel does nothing.
func refuse(refused ratelimit.Decision, taken ...ratelimit.Decision) ratelimit.Decision {
	for _, d := range taken {
		d.Cancel()
	}
	return refused
}

// both joins reservations: the one Cancel refunds every token that was taken.
func both(ds ...ratelimit.Decision) ratelimit.Decision {
	return ratelimit.Decision{OK: true, Cancel: func() {
		for _, d := range ds {
			d.Cancel()
		}
	}}
}

// ReserveSignIn takes one sign in token for the browser, then one for the
// network, then, when the request carried no valid session cookie, one of the
// shared cookieless sign in bucket (review WR-S01, IN-S05). A later refusal
// gives the earlier tokens back. The handler calls Cancel on a good sign in, so
// only wrong guesses spend the allowance.
func (g *AuthGate) ReserveSignIn(r *http.Request) ratelimit.Decision {
	if g == nil {
		return admitted()
	}
	return g.reserveDeviceNetworkShared(r, g.signInDevice, g.signInIP, g.signInNoCookie)
}

// ReserveCodeSend takes one code send token for the browser, then one for the
// network, then, when the request carried no valid session cookie, one of the
// shared cookieless bucket. A later refusal gives the earlier tokens back.
func (g *AuthGate) ReserveCodeSend(r *http.Request) ratelimit.Decision {
	if g == nil {
		return admitted()
	}
	return g.reserveDeviceNetworkShared(r, g.codeSendDevice, g.codeSendIP, g.codeSendNoCookie)
}

// reserveDeviceNetworkShared takes a device token, a network token and, for a
// request without a valid cookie, a token of the shared bucket, in that order.
func (g *AuthGate) reserveDeviceNetworkShared(r *http.Request, deviceBucket, ipBucket, sharedBucket *ratelimit.PerKey) ratelimit.Decision {
	device := deviceBucket.Reserve(deviceKey(sessionIDOf(r)))
	if !device.OK {
		return device
	}
	network := g.reserveIP(r, ipBucket)
	if !network.OK {
		return refuse(network, device)
	}
	if arrivedWithoutValidCookie(r) {
		shared := sharedBucket.Reserve(cookielessKey)
		if !shared.OK {
			return refuse(shared, device, network)
		}
		return both(device, network, shared)
	}
	return both(device, network)
}

// MailSender is the sender key of a code sending request for the mail
// ceiling's share of the reset reserve (review CR-S03): the client network
// (IPv4 address or IPv6 /64) when CLIENT_IP_HEADER is set, the one shared
// cookieless key for a request without a valid session cookie, and "" (no
// share) otherwise. It never depends on the typed address, so the share treats
// a known and an unknown address alike.
func (g *AuthGate) MailSender(r *http.Request) string {
	if g != nil && g.backstopOn {
		if key := ipBucketKey(middleware.GetClientIP(r.Context())); key != "" {
			return "net:" + key
		}
	}
	if arrivedWithoutValidCookie(r) {
		return cookielessKey
	}
	return ""
}

// withMailSender is the request context with the sender key for the mail
// ceiling.
func withMailSender(r *http.Request, gate *AuthGate) context.Context {
	return mailer.WithSender(r.Context(), gate.MailSender(r))
}

// ReserveCodeTry takes one typed code try token for the browser. There is no
// network bucket: a try needs a code row bound to the browser's own session, so
// a script that drops cookies has nothing to guess at.
func (g *AuthGate) ReserveCodeTry(r *http.Request) ratelimit.Decision {
	if g == nil {
		return admitted()
	}
	return g.codeTryDevice.Reserve(deviceKey(sessionIDOf(r)))
}

// ReserveUsernameCheck and ReserveUsernameSave take one token for the account
// (plan 10).
func (g *AuthGate) ReserveUsernameCheck(accountID int64) ratelimit.Decision {
	if g == nil {
		return admitted()
	}
	return g.usernameCheck.Reserve(accountKey(accountID))
}

func (g *AuthGate) ReserveUsernameSave(accountID int64) ratelimit.Decision {
	if g == nil {
		return admitted()
	}
	return g.usernameSave.Reserve(accountKey(accountID))
}
