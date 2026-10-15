package handlers

// ratelimit_gate.go is the one Phase 4 rate limit gate (D-10, D-11, D-12).
// Every bucket is a ratelimit.PerKey keyed on the verified account, with the
// network backstop keyed on the client IP only when a deploy setting names the
// header that carries it. Every 429 goes through writeRateLimited, and every
// message comes from package service. No limit number is typed in this file:
// the numbers live in the constants block of cmd/server/main.go and in its
// mirror api.RateLimitsDefault.

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"pinalert/internal/account"
	"pinalert/internal/ratelimit"
	"pinalert/internal/service"
)

// BucketLimit is the shape of one allowance: Burst at once, then one more every
// Every.
type BucketLimit struct {
	Every time.Duration
	Burst int
}

// RateLimits holds the seven D-11 shapes. The api package aliases this type and
// owns the defaults.
type RateLimits struct {
	// ReportSerious is the allowance for serious reports, keyed per account.
	ReportSerious BucketLimit
	// ReportOther is the allowance for all other reports, keyed per account.
	ReportOther BucketLimit
	// Vote is the allowance for votes on all five vote routes, keyed per account.
	Vote BucketLimit
	// VoteChange is the cooldown on casting the same kind of vote again on one
	// report, keyed per account, report and vote kind.
	VoteChange BucketLimit
	// Flag is the allowance for flagging reports, keyed per account.
	Flag BucketLimit
	// SubmitAttempt is the allowance for submit attempts of any outcome, keyed
	// per account, protecting the screening provider.
	SubmitAttempt BucketLimit
	// IPBackstop is the loose network address backstop, keyed on the client IP
	// and used only when a deploy setting names the header carrying it.
	IPBackstop BucketLimit
}

// RateLimitResponse is the one 429 body of every Phase 4 limiter. The
// emergency_number member is set only for a blocked serious report. Swagger
// documents it from plan 04-15.
type RateLimitResponse struct {
	Error             ErrorDetail `json:"error"`
	RetryAfterSeconds int         `json:"retry_after_seconds" example:"300"`
	Serious           bool        `json:"serious" example:"true"`
	EmergencyNumber   string      `json:"emergency_number,omitempty"`
}

const (
	voteBucket       = "vote"
	voteChangeBucket = "vote_change"
	ipBackstopBucket = "ip_backstop"

	voteRateField       = "rate_vote"
	voteChangeRateField = "rate_vote_change"

	// rateBlockLogFormat is the one line written for every block. It carries a
	// bucket name, the numeric account id and the wait, and nothing else: no
	// email, no report text and no network address (D-12).
	rateBlockLogFormat = "handlers: rate limit: bucket=%s account=%d wait_seconds=%d"
)

// RateGate holds the Phase 4 rate limit buckets. Plan 04-15 adds the report
// buckets and plan 04-20 the flag bucket inside NewRateGate.
type RateGate struct {
	vote       *ratelimit.PerKey
	voteChange *ratelimit.PerKey
	ipBackstop *ratelimit.PerKey
	backstopOn bool
	chrome     Chrome
}

// bucketConfig turns one limit into a PerKey config. It sets no idle window, so
// the floor of burst times refill applies.
func bucketConfig(l BucketLimit, clock func() time.Time) ratelimit.Config {
	return ratelimit.Config{Every: l.Every, Burst: l.Burst, Now: clock}
}

// NewRateGate builds the gate from fully resolved limits. A nil clock means the
// wall clock. The network backstop is on only when clientIPHeader is non empty.
func NewRateGate(limits RateLimits, clock func() time.Time, clientIPHeader string, chrome Chrome) *RateGate {
	if clock == nil {
		clock = time.Now
	}
	return &RateGate{
		vote:       ratelimit.NewPerKey(bucketConfig(limits.Vote, clock)),
		voteChange: ratelimit.NewPerKey(bucketConfig(limits.VoteChange, clock)),
		ipBackstop: ratelimit.NewPerKey(bucketConfig(limits.IPBackstop, clock)),
		backstopOn: strings.TrimSpace(clientIPHeader) != "",
		chrome:     chrome,
	}
}

// accountKey is the bucket key of one verified account.
func accountKey(accountID int64) string {
	return "acct:" + strconv.FormatInt(accountID, 10)
}

// voteChangeKey is the cooldown key of one account, report and vote kind. The
// report id is printed from its parsed value, so a leading zero cannot dodge it.
func voteChangeKey(accountID, reportID int64, kind service.VoteKind) string {
	return accountKey(accountID) + ":rpt:" + strconv.FormatInt(reportID, 10) + ":" + string(kind)
}

// retryAfterSeconds returns wait in whole seconds, rounded up, at least 1.
func retryAfterSeconds(wait time.Duration) int {
	if wait <= time.Second {
		return 1
	}
	return int((wait + time.Second - 1) / time.Second)
}

// reserveIPBackstop takes one token of the network backstop for the request's
// client IP. It admits the request without spending anything when the backstop
// is off or when the router found no valid client address. It never falls back
// to the connection's peer address and never shares one bucket.
func (g *RateGate) reserveIPBackstop(r *http.Request) ratelimit.Decision {
	if !g.backstopOn {
		return ratelimit.Decision{OK: true, Cancel: func() {}}
	}
	ip := middleware.GetClientIP(r.Context())
	if ip == "" {
		return ratelimit.Decision{OK: true, Cancel: func() {}}
	}
	return g.ipBackstop.Reserve(ip)
}

// writeRateLimited writes the one Phase 4 429 shape: a Retry-After header in
// whole seconds and a JSON body whose retry_after_seconds equals it. The
// emergency number is sent only when serious is true.
func writeRateLimited(w http.ResponseWriter, field, message string, wait time.Duration, serious bool, number string) {
	secs := retryAfterSeconds(wait)
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	if !serious {
		number = ""
	}
	writeJSON(w, http.StatusTooManyRequests, RateLimitResponse{
		Error:             ErrorDetail{Field: field, Message: message},
		RetryAfterSeconds: secs,
		Serious:           serious,
		EmergencyNumber:   number,
	})
}

// logBlock logs one block. It takes only a bucket name, the numeric account id
// and the wait, so it cannot carry personal data (D-12).
func logBlock(bucket string, accountID int64, wait time.Duration) {
	log.Printf(rateBlockLogFormat, bucket, accountID, retryAfterSeconds(wait))
}

// VoteMiddleware throttles one vote route. The order is the network backstop,
// then the per account vote bucket (never refunded), then the cooldown on the
// same kind of vote on the same report. A vote bucket refusal never creates a
// cooldown entry; a cooldown refusal still spends its vote token. The cooldown
// is refunded at its reservation instant whenever the wrapped handler answers
// outside 200 to 299, a panic included, so a refused vote never burns it.
func (g *RateGate) VoteMiddleware(kind service.VoteKind) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acc, ok := account.FromContext(r.Context())
			if !ok {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if d := g.reserveIPBackstop(r); !d.OK {
				logBlock(ipBackstopBucket, acc.ID, d.RetryAfter)
				writeRateLimited(w, voteRateField, service.VoteLimitMessage(d.RetryAfter), d.RetryAfter, false, "")
				return
			}

			if d := g.vote.Reserve(accountKey(acc.ID)); !d.OK {
				logBlock(voteBucket, acc.ID, d.RetryAfter)
				writeRateLimited(w, voteRateField, service.VoteLimitMessage(d.RetryAfter), d.RetryAfter, false, "")
				return
			}

			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil || id <= 0 {
				// The handler answers its own 400.
				next.ServeHTTP(w, r)
				return
			}

			change := g.voteChange.Reserve(voteChangeKey(acc.ID, id, kind))
			if !change.OK {
				logBlock(voteChangeBucket, acc.ID, change.RetryAfter)
				writeRateLimited(w, voteChangeRateField, service.VoteChangeLimitMessage(change.RetryAfter), change.RetryAfter, false, "")
				return
			}

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if status := ww.Status(); status < 200 || status > 299 {
					change.Cancel()
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
