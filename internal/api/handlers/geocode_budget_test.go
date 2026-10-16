package handlers

// Handler tests for quick task 261016-sne: the report form's address search
// has a per account budget, like the reader's place search (D-16), instead of
// the per IP middleware. Carrier NAT and a proxy without CLIENT_IP_HEADER put
// many people behind one address, so a per IP budget would make them share it.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"pinalert/internal/geocode"
	"pinalert/internal/ratelimit"
)

func TestGeocodeNoAccountAnswers500(t *testing.T) {
	stub := &stubSearcher{results: []geocode.Result{}}
	budget := &stubPlaceBudget{allow: true}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/geocode?q=Bengaluru", nil)
	Geocode(stub, budget)(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if stub.calls != 0 || len(budget.keys) != 0 {
		t.Fatalf("searcher calls %d, budget keys %v; want neither touched", stub.calls, budget.keys)
	}
}

func TestGeocodeBudgetIsPerAccount(t *testing.T) {
	budget := ratelimit.NewPerIP(time.Hour, 2)
	stub := &stubSearcher{results: []geocode.Result{}}
	handler := Geocode(stub, budget)

	status := func(accountID int64) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := geocodeRequestAs(accountID, url.Values{"q": {"Banjara Hills"}})
		// One client address for every account, as behind carrier NAT.
		req.RemoteAddr = "203.0.113.9:4444"
		handler(rec, req)
		return rec
	}

	if got := status(1).Code; got != http.StatusOK {
		t.Fatalf("account 1 first = %d, want 200", got)
	}
	if got := status(1).Code; got != http.StatusOK {
		t.Fatalf("account 1 second = %d, want 200", got)
	}
	calls := stub.calls
	refused := status(1)
	if refused.Code != http.StatusTooManyRequests {
		t.Fatalf("account 1 third = %d, want 429", refused.Code)
	}
	body := decodeErrorBody(t, refused)
	if body.Error.Field != "q" || body.Error.Message != "Too many searches. Wait a few seconds and try again." {
		t.Fatalf("error = %+v, want field q and the plain too many searches line", body.Error)
	}
	if stub.calls != calls {
		t.Fatal("a refused search reached the searcher")
	}
	if got := status(2).Code; got != http.StatusOK {
		t.Fatalf("account 2 first = %d, want 200 (the budget is per account)", got)
	}

	// The key is the account, never the client address, and never the place
	// search's own key, so the two searches keep separate budgets.
	stubBudget := &stubPlaceBudget{allow: true}
	req := geocodeRequestAs(42, url.Values{"q": {"Banjara Hills"}})
	req.RemoteAddr = "203.0.113.9:4444"
	Geocode(stub, stubBudget)(httptest.NewRecorder(), req)
	if len(stubBudget.keys) != 1 || stubBudget.keys[0] != "geocode:42" {
		t.Fatalf("budget keys = %v, want [geocode:42]", stubBudget.keys)
	}
}

// TestGeocodeBudgetEightThenOneASecond runs the shape of the new default (8 at
// once, then one a second) through the handler with a real keyed bucket.
func TestGeocodeBudgetEightThenOneASecond(t *testing.T) {
	budget := ratelimit.NewPerIP(time.Second, 8)
	handler := Geocode(&stubSearcher{results: []geocode.Result{}}, budget)
	status := func() int {
		rec := httptest.NewRecorder()
		handler(rec, geocodeRequestAs(5, url.Values{"q": {"Banjara Hills"}}))
		return rec.Code
	}
	for i := 1; i <= 8; i++ {
		if got := status(); got != http.StatusOK {
			t.Fatalf("search %d = %d, want 200", i, got)
		}
	}
	if got := status(); got != http.StatusTooManyRequests {
		t.Fatalf("search 9 = %d, want 429", got)
	}
	time.Sleep(1100 * time.Millisecond)
	if got := status(); got != http.StatusOK {
		t.Fatalf("search after a second = %d, want 200", got)
	}
	if got := status(); got != http.StatusTooManyRequests {
		t.Fatalf("the next search = %d, want 429 (one a second)", got)
	}
}

// TestGeocodeNilBudgetSkipsTheBudget keeps the nil searcher and nil budget
// paths safe for a Deps literal that leaves them out.
func TestGeocodeNilBudgetSkipsTheBudget(t *testing.T) {
	rec := httptest.NewRecorder()
	Geocode(&stubSearcher{results: []geocode.Result{}}, nil)(rec, geocodeRequestAs(5, url.Values{"q": {"Banjara Hills"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	rec = httptest.NewRecorder()
	Geocode(nil, &stubPlaceBudget{allow: false})(rec, geocodeRequestAs(5, url.Values{"q": {"Banjara Hills"}}))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("nil searcher with a refused budget = %d, want 429", rec.Code)
	}
}
