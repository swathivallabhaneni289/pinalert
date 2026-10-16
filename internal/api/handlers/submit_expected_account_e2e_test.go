// submit_expected_account_e2e_test.go covers WR-W09 of the Phase 4 web review:
// a saved report the device queue posts names the account it was saved under in
// expected_account_id, and the server refuses it with 409 on that field when a
// different account is signed in. Every package level identifier carries the
// pw9 prefix. These tests run only on a disposable database, never on pinalert
// or pinalert_test.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"pinalert/internal/api"
)

const pw9Field = "expected_account_id"

// pw9Setup starts a server whose screener counts its calls and whose other
// report allowance is one report an hour, and verifies two accounts.
func pw9Setup(t *testing.T) (srvURL string, a, b *http.Client, aID, bID int64, scr *p17CountingScreener, countAll func() int) {
	t.Helper()
	clk := &p17Clock{now: p17T0}
	scr = &p17CountingScreener{}
	srv, mailer, pool := newE2EServerWith(t, p17Tweak(clk, scr, func(d *api.Deps) {
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 1}
	}))
	a = newVerifiedClient(t, srv, mailer, "pw9-a@example.com")
	b = newVerifiedClient(t, srv, mailer, "pw9-b@example.com")
	aID = p17AccountID(t, pool, a, srv.URL)
	bID = p17AccountID(t, pool, b, srv.URL)
	if aID == bID {
		t.Fatalf("the two accounts share the id %d", aID)
	}
	return srv.URL, a, b, aID, bID, scr, func() int { return p17CountAll(t, pool) }
}

// pw9RequireRefusal checks a 409 body: the field is expected_account_id, the
// message is the one plain sentence, and the body carries no other key, so it
// names neither account.
func pw9RequireRefusal(t *testing.T, raw []byte) {
	t.Helper()
	field, message := p17ErrorField(t, raw)
	if field != pw9Field {
		t.Fatalf("field = %q, want %q (body %s)", field, pw9Field, raw)
	}
	want := "This report was saved under a different account than the one signed in now. Sign in to that account to send it."
	if message != want {
		t.Fatalf("message = %q, want %q", message, want)
	}
	var top map[string]map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
	if len(top) != 1 || len(top["error"]) != 2 {
		t.Fatalf("the 409 body holds more than error.field and error.message: %s", raw)
	}
}

func TestSubmitExpectedAccountMatchIsStored(t *testing.T) {
	url, a, _, aID, _, _, countAll := pw9Setup(t)
	body := p17Body("flood", "low", p17Desc, map[string]any{pw9Field: strconv.FormatInt(aID, 10)})
	p17Expect(t, "matching account", a, url, body, http.StatusCreated)
	if n := countAll(); n != 1 {
		t.Fatalf("stored reports = %d, want 1", n)
	}
}

func TestSubmitExpectedAccountMissingIsStored(t *testing.T) {
	url, a, _, _, _, _, countAll := pw9Setup(t)
	p17Expect(t, "no expected account", a, url, p17Body("flood", "low", p17Desc, nil), http.StatusCreated)
	if n := countAll(); n != 1 {
		t.Fatalf("stored reports = %d, want 1", n)
	}
}

func TestSubmitExpectedAccountMismatchIsRefused(t *testing.T) {
	url, a, b, aID, _, scr, countAll := pw9Setup(t)
	saved := strconv.FormatInt(aID, 10)

	// A report saved under A is posted with B's cookie: refused, nothing
	// stored, the text never checked.
	raw := p17Expect(t, "saved under A, sent as B", b, url,
		p17Body("flood", "low", p17Desc, map[string]any{pw9Field: saved}), http.StatusConflict)
	pw9RequireRefusal(t, raw)
	if n := countAll(); n != 0 {
		t.Fatalf("stored reports after the refusal = %d, want 0", n)
	}
	if scr.Calls() != 0 {
		t.Fatalf("screener calls after the refusal = %d, want 0", scr.Calls())
	}

	// An otherwise invalid body still gets the 409, never a 400 the queue would
	// turn into an Edit and resend under the wrong account.
	raw = p17Expect(t, "invalid and mismatched", b, url,
		p17Body("flood", "low", "", map[string]any{pw9Field: saved}), http.StatusConflict)
	pw9RequireRefusal(t, raw)

	// Any value that is not exactly the signed in id is refused.
	for _, v := range []string{"", "0", "abc", "0" + saved, saved + " "} {
		raw = p17Expect(t, "value "+strconv.Quote(v), a, url,
			p17Body("flood", "low", p17Desc, map[string]any{pw9Field: v}), http.StatusConflict)
		pw9RequireRefusal(t, raw)
	}

	// The refusals used none of B's allowance of one report an hour.
	p17Expect(t, "B's own report", b, url, p17Body("flood", "low", p17Desc, nil), http.StatusCreated)
	if n := countAll(); n != 1 {
		t.Fatalf("stored reports = %d, want 1", n)
	}
}

func TestSubmitExpectedAccountMismatchBeatsReplay(t *testing.T) {
	url, a, b, aID, bID, _, countAll := pw9Setup(t)
	const key = "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c99"

	// B already used the key, so a plain retry by B is a 200 replay.
	p17Expect(t, "B stores the key", b, url,
		p17Body("flood", "low", p17Desc, map[string]any{"request_id": key, pw9Field: strconv.FormatInt(bID, 10)}),
		http.StatusCreated)

	// A report saved under A with that same key, sent with B's cookie, is
	// refused before the replay lookup, so it is never answered with B's report.
	raw := p17Expect(t, "saved under A, B's key", b, url,
		p17Body("flood", "low", p17Desc, map[string]any{"request_id": key, pw9Field: strconv.FormatInt(aID, 10)}),
		http.StatusConflict)
	pw9RequireRefusal(t, raw)
	if n := countAll(); n != 1 {
		t.Fatalf("stored reports = %d, want 1", n)
	}

	// A itself can still send it.
	p17Expect(t, "A sends its own", a, url,
		p17Body("flood", "low", p17Desc, map[string]any{"request_id": key, pw9Field: strconv.FormatInt(aID, 10)}),
		http.StatusCreated)
}
