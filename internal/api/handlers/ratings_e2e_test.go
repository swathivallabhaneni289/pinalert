// ratings_e2e_test.go proves the two rating routes of Phase 9 plan 14 end to end
// over the real router and a real Postgres (D-02, D-23, UI-SPEC 3a, V6, T-09-53,
// T-09-54): who may rate, the once a day change rule, one fixed sentence for every
// refusal so the route is not an oracle, the uniform 404 for a person the caller
// may not see, strict bodies, and the per account Rate bucket. Every package
// level identifier added here carries the p914r prefix.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// p914rProfile reads the rating fields of GET /api/people/{username} as seen by
// client.
func p914rProfile(t *testing.T, client *http.Client, srv *httptest.Server, username string) (mayRate bool, yourRating int) {
	t.Helper()
	body, obj := p907Get(t, client, srv, "/api/people/"+username, http.StatusOK)
	may, ok := obj["may_rate"].(bool)
	if !ok {
		t.Fatalf("GET /api/people/%s has no boolean may_rate: %s", username, body)
	}
	yours, ok := obj["your_rating"].(float64)
	if !ok {
		t.Fatalf("GET /api/people/%s has no numeric your_rating: %s", username, body)
	}
	return may, int(yours)
}

// p914rRate POSTs a rating body and returns the status and the body.
func p914rRate(t *testing.T, client *http.Client, srv *httptest.Server, username, body string) (int, string) {
	t.Helper()
	status, raw, _ := p907Do(t, client, http.MethodPost, srv.URL+"/api/people/"+username+"/rating", body)
	return status, raw
}

// p914rClear POSTs the clear route with no body and returns the status and body.
func p914rClear(t *testing.T, client *http.Client, srv *httptest.Server, username string) (int, string) {
	t.Helper()
	status, raw, _ := p907Do(t, client, http.MethodPost, srv.URL+"/api/people/"+username+"/rating/clear", "")
	return status, raw
}

// p914rWantRefusal requires a 409 whose field is stars and whose sentence is want.
func p914rWantRefusal(t *testing.T, label string, status int, body, want string) {
	t.Helper()
	if status != http.StatusConflict {
		t.Errorf("%s: status = %d, want 409, body %s", label, status, body)
		return
	}
	if field, message := p907ErrorOf(t, body); field != "stars" || message != want {
		t.Errorf("%s: field %q message %q, want stars and %q", label, field, message, want)
	}
}

// TestRatingRoutes is V6 (D-02, D-23, UI-SPEC 3a, T-09-53, T-09-54).
func TestRatingRoutes(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
		d.RateLimits.Rate = e2eGenerousBucket
	})
	ctx := context.Background()

	reporter, reporterID := p907Reporter(t, srv, mailer, pool, "p914r-reporter@example.com", "rr.reporter")
	report := testutil.SeedHistoryReport(t, pool, "sess-rr.reporter", "low", "flood", time.Now().UTC().Add(time.Hour))

	// An eligible rater confirms the report through the API.
	rater := newVerifiedClient(t, srv, mailer, "p914r-rater@example.com")
	if status, raw, err := p07PostVoteRaw(rater, srv.URL, report, "confirm", p07ReportLat, p07ReportLon); err != nil || status != http.StatusOK {
		t.Fatalf("the rater's confirm: status %d, err %v, body %s", status, err, raw)
	}
	var raterID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM accounts WHERE email = $1", "p914r-rater@example.com").Scan(&raterID); err != nil {
		t.Fatalf("reading the rater: %v", err)
	}

	// An account that never voted, and one whose only vote is an unresolve.
	stranger := newVerifiedClient(t, srv, mailer, "p914r-stranger@example.com")
	takeBack := newVerifiedClient(t, srv, mailer, "p914r-takeback@example.com")
	var takeBackID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM accounts WHERE email = $1", "p914r-takeback@example.com").Scan(&takeBackID); err != nil {
		t.Fatalf("reading the take back account: %v", err)
	}
	testutil.MustExec(t, pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'resolution', 'unresolve', 'tdr1v8y')`,
		report, takeBackID)

	// The profile tells the reader whether they may rate, and their own rating.
	if may, yours := p914rProfile(t, rater, srv, "rr.reporter"); !may || yours != 0 {
		t.Fatalf("before rating: may_rate %v your_rating %d, want true and 0", may, yours)
	}
	if may, yours := p914rProfile(t, stranger, srv, "rr.reporter"); may || yours != 0 {
		t.Errorf("a stranger: may_rate %v your_rating %d, want false and 0", may, yours)
	}
	if may, yours := p914rProfile(t, reporter, srv, "rr.reporter"); may || yours != 0 {
		t.Errorf("the reporter's own page: may_rate %v your_rating %d, want false and 0", may, yours)
	}

	// Rate 4: 204, and the profile reads it back.
	if status, raw := p914rRate(t, rater, srv, "rr.reporter", `{"stars":4}`); status != http.StatusNoContent {
		t.Fatalf("rating 4: status %d, body %s", status, raw)
	}
	if may, yours := p914rProfile(t, rater, srv, "rr.reporter"); !may || yours != 4 {
		t.Errorf("after rating 4: may_rate %v your_rating %d, want true and 4", may, yours)
	}
	var stored int
	if err := pool.QueryRow(ctx,
		"SELECT stars FROM profile_ratings WHERE rater_id = $1 AND reporter_id = $2", raterID, reporterID).Scan(&stored); err != nil || stored != 4 {
		t.Errorf("stored rating = %d (%v), want 4", stored, err)
	}

	// A different value at once is refused with the change limit sentence; the
	// same value again changes nothing and is fine.
	status, raw := p914rRate(t, rater, srv, "rr.reporter", `{"stars":5}`)
	p914rWantRefusal(t, "rating 5 at once", status, raw, service.RatingTooSoonMessage)
	if service.RatingTooSoonMessage != "You can change this rating again tomorrow." {
		t.Errorf("the change limit sentence = %q", service.RatingTooSoonMessage)
	}
	if status, raw := p914rRate(t, rater, srv, "rr.reporter", `{"stars":4}`); status != http.StatusNoContent {
		t.Errorf("rating 4 again: status %d, body %s, want 204", status, raw)
	}

	// Every reason a rating is not allowed answers the same 409, byte for byte:
	// the reporter rating themselves, an account that never voted, and an account
	// whose only vote is an unresolve.
	var refusals []string
	for label, client := range map[string]*http.Client{"yourself": reporter, "never voted": stranger, "only an unresolve": takeBack} {
		status, raw := p914rRate(t, client, srv, "rr.reporter", `{"stars":4}`)
		p914rWantRefusal(t, label, status, raw, service.RatingNotAllowedMessage)
		refusals = append(refusals, raw)
	}
	for _, other := range refusals[1:] {
		if other != refusals[0] {
			t.Errorf("the refusals differ, so the route is an oracle: %q and %q", refusals[0], other)
		}
	}

	// Stars outside 1 to 5 and a body that is not {"stars": n} are 400 on stars.
	for _, bad := range []string{`{"stars":6}`, `{"stars":0}`, `{"stars":-1}`, `{}`, `{"stars":"4"}`, `{"stars":4.5}`, `{"stars":4,"extra":1}`, `not json`} {
		status, raw := p914rRate(t, rater, srv, "rr.reporter", bad)
		if status != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400, body %s", bad, status, raw)
			continue
		}
		if field, message := p907ErrorOf(t, raw); field != "stars" || message != service.RatingStarsMessage {
			t.Errorf("body %q: field %q message %q, want stars and %q", bad, field, message, service.RatingStarsMessage)
		}
	}

	// Clearing inside the day is refused with the same change limit sentence.
	status, raw = p914rClear(t, rater, srv, "rr.reporter")
	p914rWantRefusal(t, "clear inside the day", status, raw, service.RatingTooSoonMessage)
	if may, yours := p914rProfile(t, rater, srv, "rr.reporter"); !may || yours != 4 {
		t.Errorf("after a refused clear: may_rate %v your_rating %d, want true and 4", may, yours)
	}

	// A day later the rating can be cleared, and the profile reads 0 again.
	testutil.MustExec(t, pool, `UPDATE profile_ratings SET updated_at = now() - interval '25 hours' WHERE rater_id = $1`, raterID)
	if status, raw := p914rClear(t, rater, srv, "rr.reporter"); status != http.StatusNoContent {
		t.Fatalf("clear a day later: status %d, body %s", status, raw)
	}
	if may, yours := p914rProfile(t, rater, srv, "rr.reporter"); !may || yours != 0 {
		t.Errorf("after clearing: may_rate %v your_rating %d, want true and 0", may, yours)
	}

	// A person the caller may not see answers the uniform 404, the same bytes as a
	// username that does not exist.
	testutil.MustExec(t, pool, `UPDATE accounts SET profile_public = false WHERE id = $1`, reporterID)
	status, privateBody := p914rRate(t, rater, srv, "rr.reporter", `{"stars":3}`)
	if status != http.StatusNotFound {
		t.Fatalf("rating a private profile: status %d, want 404, body %s", status, privateBody)
	}
	status, unknownBody := p914rRate(t, rater, srv, "rr.nobody", `{"stars":3}`)
	if status != http.StatusNotFound || unknownBody != privateBody {
		t.Errorf("an unknown username: status %d body %q, want 404 with the same body as %q", status, unknownBody, privateBody)
	}
	// Withdrawing your own rating never needs the person to be visible, so a clear
	// of a private profile is a quiet 204 that changes nothing here.
	if status, raw := p914rClear(t, rater, srv, "rr.reporter"); status != http.StatusNoContent {
		t.Errorf("clearing a private profile: status %d, body %s, want 204", status, raw)
	}
}

// TestRatingRateLimit is V6 (T-09-54): with a small Rate bucket the next try
// answers 429 with the rate_rating field, the limit is per account, and the 24
// hour rule is a second, separate wall.
func TestRatingRateLimit(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.PeopleRead = e2eGenerousBucket
		d.RateLimits.PeopleAction = e2eGenerousBucket
		d.RateLimits.Rate = api.BucketLimit{Every: time.Hour, Burst: 2}
	})

	_, _ = p907Reporter(t, srv, mailer, pool, "p914r-limit-reporter@example.com", "rl.reporter")
	report := testutil.SeedHistoryReport(t, pool, "sess-rl.reporter", "low", "flood", time.Now().UTC().Add(time.Hour))
	rater := newVerifiedClient(t, srv, mailer, "p914r-limit-rater@example.com")
	other := newVerifiedClient(t, srv, mailer, "p914r-limit-other@example.com")
	for _, c := range []*http.Client{rater, other} {
		if status, raw, err := p07PostVoteRaw(c, srv.URL, report, "confirm", p07ReportLat, p07ReportLon); err != nil || status != http.StatusOK {
			t.Fatalf("a confirm: status %d, err %v, body %s", status, err, raw)
		}
	}

	for i := 0; i < 2; i++ {
		if status, raw := p914rRate(t, rater, srv, "rl.reporter", `{"stars":4}`); status != http.StatusNoContent {
			t.Fatalf("try %d: status %d, body %s, want 204", i+1, status, raw)
		}
	}
	status, raw, header := p907Do(t, rater, http.MethodPost, srv.URL+"/api/people/rl.reporter/rating", `{"stars":4}`)
	if status != http.StatusTooManyRequests {
		t.Fatalf("third try: status %d, body %s, want 429", status, raw)
	}
	var limited struct {
		Error struct {
			Field string `json:"field"`
		} `json:"error"`
		RetryAfterSeconds int `json:"retry_after_seconds"`
	}
	if err := json.Unmarshal([]byte(raw), &limited); err != nil || limited.Error.Field != "rate_rating" || limited.RetryAfterSeconds < 1 {
		t.Errorf("429 body = %s (%v), want the rate_rating field and a retry time", raw, err)
	}
	if header.Get("Retry-After") == "" {
		t.Errorf("the 429 has no Retry-After header")
	}
	// The clear route shares the bucket.
	if status, raw := p914rClear(t, rater, srv, "rl.reporter"); status != http.StatusTooManyRequests {
		t.Errorf("clear after the bucket is empty: status %d, body %s, want 429", status, raw)
	}
	// The limit is per account: another account is not refused.
	if status, raw := p914rRate(t, other, srv, "rl.reporter", `{"stars":3}`); status != http.StatusNoContent {
		t.Errorf("another account: status %d, body %s, want 204", status, raw)
	}
}
