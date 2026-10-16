package geocode

// Tests for F-P4-11: a first Photon try that runs out of its time limit gets
// one retry, with a longer limit, before the search falls back to Nominatim.
// The retry counts against the cap of four upstream requests, takes a Photon
// token only if one is free right now (so it never waits and never queues
// ahead of a report search, D-16) and is the only retry: a fast failure, a
// shorter text and the retry itself are never tried again. Every upstream
// here is an httptest stand in; nothing calls the real Photon or Nominatim.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

// hangThen is a Photon responder that gives no answer to its first hangs
// requests (it waits until the request is given up, at most 3 seconds) and
// answers every later one with then.
func hangThen(hangs int, then func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	var mu sync.Mutex
	seen := 0
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen++
		n := seen
		mu.Unlock()
		if n <= hangs {
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
			}
			return
		}
		then(w, r)
	}
}

// newRetryClient is newDualClient with time limits short enough for a test.
func newRetryClient(photonURL, nominatimURL string) *Client {
	c := newDualClient(photonURL, nominatimURL)
	c.photonTimeout = 100 * time.Millisecond
	c.photonRetryTimeout = 300 * time.Millisecond
	return c
}

func TestSlowFirstPhotonTryGetsOneRetry(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, hangThen(1, photonAnswers(t, photonFeatureJSON(78.4347, 17.4156, map[string]any{
				"name": "Banjara Hills", "city": "Hyderabad", "state": "Telangana", "countrycode": "IN",
				"osm_key": "place", "type": "district",
			}))))
			rs := newRecordingServer(t, func(url.Values) bool { return true })
			c := newRetryClient(ps.server.URL, rs.server.URL)
			near := &Point{Lat: 17.385, Lon: 78.4867}

			start := time.Now()
			answer, err := search(c, context.Background(), "Banjra Hills", near)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 2*time.Second {
				t.Errorf("search took %v, want about the first time limit plus the answer", elapsed)
			}
			if ps.count() != 2 {
				t.Fatalf("photon got %d requests, want 2 (the slow first try and its retry)", ps.count())
			}
			if n := len(rs.sent()); n != 0 {
				t.Fatalf("nominatim got %d requests, want 0 when the retry found the place", n)
			}
			ps.mu.Lock()
			first, second := ps.queries[0], ps.queries[1]
			ps.mu.Unlock()
			if !reflect.DeepEqual(first, second) {
				t.Errorf("retry query = %v, want the first try's query %v", second, first)
			}
			if answer.provider != providerPhoton || len(answer.Results) != 1 || answer.Results[0].Name != "Banjara Hills" || answer.MatchedQuery != "" {
				t.Fatalf("answer = %+v, want Banjara Hills from photon", answer)
			}
		})
	}
}

// TestPhotonRetryCountsAgainstTheCap pins the cap with the retry: a Photon
// that hangs twice leaves two Nominatim tries, the boxed and the widened
// whole text, and the empty answer is not cached.
func TestPhotonRetryCountsAgainstTheCap(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, hangThen(10, nil))
			rs := newRecordingServer(t, nil)
			c := newRetryClient(ps.server.URL, rs.server.URL)
			near := &Point{Lat: 17.4, Lon: 78.45}

			answer, err := search(c, context.Background(), longAddress, near)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			sent := rs.sent()
			if ps.count() != 2 || len(sent) != 2 {
				t.Fatalf("photon %d + nominatim %d requests, want 2 + 2", ps.count(), len(sent))
			}
			if sent[0].Get("q") != longAddress || sent[0].Get("bounded") != "1" ||
				sent[1].Get("q") != longAddress || sent[1].Has("bounded") {
				t.Fatalf("nominatim tries = %v, want the whole text boxed, then widened", sent)
			}
			if answer.Results == nil || len(answer.Results) != 0 {
				t.Fatalf("answer = %+v, want an empty non nil list", answer)
			}
			if c.reporter.size() != 0 || c.reader.size() != 0 || c.photon.size() != 0 {
				t.Fatalf("caches hold %d, %d and %d entries, want none after photon failed",
					c.reporter.size(), c.reader.size(), c.photon.size())
			}
		})
	}
}

// TestPhotonRetryThatFindsNothingRunsTheOrder: a retry that answers with
// nothing takes the first try's place, so the 261016-sfu order runs with the
// two requests the cap leaves: Nominatim with the whole text, then Photon
// with the first shorter text.
func TestPhotonRetryThatFindsNothingRunsTheOrder(t *testing.T) {
	short1 := "Sai Residency, Road No 12, Banjara Hills, Hyderabad"
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, hangThen(1, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("q") == short1 {
					photonAnswers(t, indianCity("Road No 12"))(w, r)
					return
				}
				photonAnswers(t)(w, r)
			}))
			rs := newRecordingServer(t, nil)
			c := newRetryClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), longAddress, nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if got, want := rs.texts(), []string{longAddress}; !reflect.DeepEqual(got, want) {
				t.Fatalf("nominatim texts = %q, want %q", got, want)
			}
			if ps.count() != 3 || ps.last().Get("q") != short1 {
				t.Fatalf("photon got %d requests ending with %q, want 3 ending with the first shorter text", ps.count(), ps.last().Get("q"))
			}
			if answer.MatchedQuery != short1 || len(answer.Results) != 1 || answer.provider != providerPhoton {
				t.Fatalf("answer = %+v, want the photon answer matched on %q", answer, short1)
			}
		})
	}
}

// TestPhotonRetryNeverWaitsForAToken: with no Photon token free the retry is
// skipped at once and the search falls back, on the report search too, so the
// retry adds no limiter wait.
func TestPhotonRetryNeverWaitsForAToken(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, hangThen(10, nil))
			rs := newRecordingServer(t, func(url.Values) bool { return true })
			c := newClientWithProviders(ps.server.URL, rs.server.URL, "", time.Millisecond, 10, 10*time.Second, 1)
			c.photonTimeout = 100 * time.Millisecond

			start := time.Now()
			answer, err := search(c, context.Background(), "Banjara Hills", nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
				t.Errorf("search took %v, want no wait for a retry token", elapsed)
			}
			if ps.count() != 1 || len(rs.sent()) != 1 || answer.provider != providerNominatim {
				t.Fatalf("photon %d, nominatim %d, provider %q; want 1, 1 and nominatim", ps.count(), len(rs.sent()), answer.provider)
			}
		})
	}
}

// TestReaderPhotonRetryNeverQueuesAheadOfAReporter is D-16 for the retry:
// while a report search waits for Photon's next token, a place search whose
// first try ran out of time does not take that token for its retry. It falls
// back to Nominatim as before, and the report search is answered.
func TestReaderPhotonRetryNeverQueuesAheadOfAReporter(t *testing.T) {
	ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "reader text" {
			hangThen(10, nil)(w, r)
			return
		}
		photonAnswers(t, indianCity("Hyderabad"))(w, r)
	})
	rs := newRecordingServer(t, func(url.Values) bool { return true })
	c := newClientWithProviders(ps.server.URL, rs.server.URL, "", time.Millisecond, 10, 400*time.Millisecond, 1)
	c.photonTimeout = 150 * time.Millisecond

	readerDone := make(chan error, 1)
	var readerAnswer Answer
	go func() {
		var err error
		readerAnswer, err = c.SearchReader(context.Background(), "reader text", nil)
		readerDone <- err
	}()
	// The reader has taken the only token and is waiting on Photon. A
	// reporter now waits for the next one, which puts the bucket in debt.
	time.Sleep(20 * time.Millisecond)
	reporterDone := make(chan error, 1)
	go func() {
		_, err := c.Search(context.Background(), "Hyderabad", nil)
		reporterDone <- err
	}()

	for _, done := range []chan error{readerDone, reporterDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("a search was never answered")
		}
	}
	if ps.count() != 2 {
		t.Fatalf("photon got %d requests, want 2 (the reader's first try and the reporter's), no reader retry", ps.count())
	}
	if readerAnswer.provider != providerNominatim {
		t.Fatalf("reader answer provider = %q, want nominatim", readerAnswer.provider)
	}
}

func TestCancelledSearchDuringPhotonRetryDoesNotFallBack(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var mu sync.Mutex
			seen := 0
			ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				seen++
				n := seen
				mu.Unlock()
				if n == 2 {
					cancel()
				}
				select {
				case <-r.Context().Done():
				case <-time.After(3 * time.Second):
				}
			})
			rs := newRecordingServer(t, nil)
			c := newRetryClient(ps.server.URL, rs.server.URL)

			_, err := search(c, ctx, "Banjara Hills", nil)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if ps.count() != 2 {
				t.Fatalf("photon got %d requests, want 2", ps.count())
			}
			if n := len(rs.sent()); n != 0 {
				t.Fatalf("nominatim got %d requests after the search was cancelled, want 0", n)
			}
		})
	}
}
