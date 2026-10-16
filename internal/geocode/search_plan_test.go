package geocode

// Tests for quick task 261016-adr: India only, near the caller first, fewer
// words when the whole text finds nothing, and the honest matched text. Every
// upstream here is an httptest stand in; nothing calls the real Nominatim.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"
)

const longAddress = "Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad"

// recordingServer is an upstream that records every query in order and
// answers with one record only when match says so, else with [].
type recordingServer struct {
	mu      sync.Mutex
	queries []url.Values
	server  *httptest.Server
}

func newRecordingServer(t *testing.T, match func(q url.Values) bool) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		rs.mu.Lock()
		rs.queries = append(rs.queries, q)
		rs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if match != nil && match(q) {
			_, _ = w.Write([]byte(`[{"name":"Banjara Hills","display_name":"Banjara Hills, Hyderabad, Telangana, India","lat":"17.4156","lon":"78.4347"}]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(rs.server.Close)
	return rs
}

func (rs *recordingServer) sent() []url.Values {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]url.Values(nil), rs.queries...)
}

func (rs *recordingServer) texts() []string {
	var out []string
	for _, q := range rs.sent() {
		out = append(out, q.Get("q"))
	}
	return out
}

func TestSearchTextsDropTheMostSpecificPart(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{longAddress, []string{
			longAddress,
			"Sai Residency, Road No 12, Banjara Hills, Hyderabad",
			"Road No 12, Banjara Hills, Hyderabad",
		}},
		{"Near Charminar bus stop", []string{"Near Charminar bus stop", "Charminar bus stop", "bus stop"}},
		{"  Flat 2 ,, Banjara  Hills  ", []string{"Flat 2 ,, Banjara  Hills", "Banjara Hills", "Hills"}},
		{"Hyderabad", []string{"Hyderabad"}},
		{"12, MG", []string{"12, MG"}},
		{"ab cd", []string{"ab cd"}},
	}
	for _, tc := range cases {
		if got := searchTexts(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("searchTexts(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSearchPlanOrderAndCap(t *testing.T) {
	near := &Point{Lat: 17.4, Lon: 78.45}
	type step struct {
		text  string
		boxed bool
	}
	steps := func(plan []attempt) []step {
		var out []step
		for _, a := range plan {
			out = append(out, step{a.text, a.boxed})
		}
		return out
	}

	withNear := steps(searchPlan(longAddress, near))
	wantNear := []step{
		{longAddress, true},
		{longAddress, false},
		{"Sai Residency, Road No 12, Banjara Hills, Hyderabad", true},
		{"Sai Residency, Road No 12, Banjara Hills, Hyderabad", false},
	}
	if !reflect.DeepEqual(withNear, wantNear) {
		t.Errorf("plan with a position = %v, want %v", withNear, wantNear)
	}
	if len(withNear) != maxUpstreamPerSearch || maxUpstreamPerSearch != 4 {
		t.Errorf("plan has %d steps and the cap is %d, want both 4", len(withNear), maxUpstreamPerSearch)
	}

	noNear := steps(searchPlan(longAddress, nil))
	wantNoNear := []step{
		{longAddress, false},
		{"Sai Residency, Road No 12, Banjara Hills, Hyderabad", false},
		{"Road No 12, Banjara Hills, Hyderabad", false},
	}
	if !reflect.DeepEqual(noNear, wantNoNear) {
		t.Errorf("plan without a position = %v, want %v", noNear, wantNoNear)
	}

	oneWord := steps(searchPlan("Hyderabad", near))
	if want := []step{{"Hyderabad", true}, {"Hyderabad", false}}; !reflect.DeepEqual(oneWord, want) {
		t.Errorf("plan for one word = %v, want %v", oneWord, want)
	}
}

// Both lanes run the same plan, so every behaviour below is checked on both.
type laneFunc func(c *Client, ctx context.Context, q string, near *Point) (Answer, error)

var lanes = map[string]laneFunc{
	"reporter": func(c *Client, ctx context.Context, q string, near *Point) (Answer, error) {
		return c.Search(ctx, q, near)
	},
	"reader": func(c *Client, ctx context.Context, q string, near *Point) (Answer, error) {
		return c.SearchReader(ctx, q, near)
	},
}

func TestEveryRequestIsIndiaOnly(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			rs := newRecordingServer(t, nil)
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			if _, err := search(c, context.Background(), longAddress, &Point{Lat: 17.4, Lon: 78.45}); err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			sent := rs.sent()
			if len(sent) != 4 {
				t.Fatalf("sent %d requests, want 4", len(sent))
			}
			for i, q := range sent {
				if q.Get("countrycodes") != "in" {
					t.Errorf("request %d countrycodes = %q, want in", i, q.Get("countrycodes"))
				}
			}
		})
	}
}

func TestBoxFirstThenWidened(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			// Only the widened request finds anything.
			rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("viewbox") == "" })
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			answer, err := search(c, context.Background(), "Hyderabad", &Point{Lat: 28.61, Lon: 77.21})
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			sent := rs.sent()
			if len(sent) != 2 {
				t.Fatalf("sent %d requests, want 2", len(sent))
			}
			if sent[0].Get("bounded") != "1" || sent[0].Get("viewbox") != "76.710000,29.110000,77.710000,28.110000" {
				t.Errorf("first request = %v, want the bounded box around the position", sent[0])
			}
			if _, has := sent[1]["viewbox"]; has {
				t.Errorf("widened request still has a viewbox: %v", sent[1])
			}
			if _, has := sent[1]["bounded"]; has {
				t.Errorf("widened request still has bounded: %v", sent[1])
			}
			if sent[0].Get("q") != "Hyderabad" || sent[1].Get("q") != "Hyderabad" {
				t.Errorf("texts = %q, want the full text twice", rs.texts())
			}
			if len(answer.Results) != 1 || answer.MatchedQuery != "" {
				t.Errorf("answer = %+v, want one result and no matched text", answer)
			}
		})
	}
}

func TestShorterTriesInOrderStopAtFirstHit(t *testing.T) {
	short1 := "Sai Residency, Road No 12, Banjara Hills, Hyderabad"
	short2 := "Road No 12, Banjara Hills, Hyderabad"
	for name, search := range lanes {
		t.Run(name+" second shorter try", func(t *testing.T) {
			rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("q") == short2 })
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			answer, err := search(c, context.Background(), longAddress, nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if got, want := rs.texts(), []string{longAddress, short1, short2}; !reflect.DeepEqual(got, want) {
				t.Fatalf("texts sent = %q, want %q", got, want)
			}
			if answer.MatchedQuery != short2 || len(answer.Results) != 1 {
				t.Errorf("answer = %+v, want one result matched on %q", answer, short2)
			}
		})
		t.Run(name+" stops at the first hit", func(t *testing.T) {
			rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("q") == short1 })
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			answer, err := search(c, context.Background(), longAddress, nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if got, want := rs.texts(), []string{longAddress, short1}; !reflect.DeepEqual(got, want) {
				t.Fatalf("texts sent = %q, want %q", got, want)
			}
			if answer.MatchedQuery != short1 {
				t.Errorf("MatchedQuery = %q, want %q", answer.MatchedQuery, short1)
			}
		})
		t.Run(name+" full text match has no matched text", func(t *testing.T) {
			rs := newRecordingServer(t, func(q url.Values) bool { return true })
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			answer, err := search(c, context.Background(), "  "+longAddress+" ", nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if len(rs.sent()) != 1 || answer.MatchedQuery != "" {
				t.Errorf("sent %d, MatchedQuery %q; want 1 and empty", len(rs.sent()), answer.MatchedQuery)
			}
		})
	}
}

func TestRequestCapIsFour(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			rs := newRecordingServer(t, nil)
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			answer, err := search(c, context.Background(), longAddress, &Point{Lat: 17.4, Lon: 78.45})
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if n := len(rs.sent()); n != 4 {
				t.Fatalf("sent %d requests, want the cap of 4", n)
			}
			if answer.Results == nil || len(answer.Results) != 0 || answer.MatchedQuery != "" {
				t.Errorf("answer = %+v, want an empty non nil list and no matched text", answer)
			}
		})
	}
}

func TestFinalAnswerCachedUnderTheOriginalTextAndArea(t *testing.T) {
	// With a position the plan reaches the first shorter text only.
	short1 := "Sai Residency, Road No 12, Banjara Hills, Hyderabad"
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("q") == short1 })
			c := newClientWithBaseURL(rs.server.URL, "", time.Millisecond, 10)
			near := &Point{Lat: 17.40, Lon: 78.45}
			if _, err := search(c, context.Background(), longAddress, near); err != nil {
				t.Fatalf("first search returned error: %v", err)
			}
			before := len(rs.sent())

			// Same text in other case and spacing, same 0.1 degree area.
			again, err := search(c, context.Background(), "  flat 4B,  Sai Residency, Road No 12, Banjara Hills, HYDERABAD", &Point{Lat: 17.41, Lon: 78.46})
			if err != nil {
				t.Fatalf("repeat search returned error: %v", err)
			}
			if n := len(rs.sent()); n != before {
				t.Fatalf("repeat search sent %d more requests, want 0", n-before)
			}
			if again.MatchedQuery != short1 || len(again.Results) != 1 {
				t.Errorf("cached answer = %+v, want the matched text kept", again)
			}

			// The shorter text itself is not a cache key: only the original is.
			if _, err := search(c, context.Background(), short1, near); err != nil {
				t.Fatalf("shorter text search returned error: %v", err)
			}
			if n := len(rs.sent()); n == before {
				t.Errorf("searching the shorter text was answered from the cache")
			}

			// Another area is another entry.
			before = len(rs.sent())
			if _, err := search(c, context.Background(), longAddress, &Point{Lat: 12.97, Lon: 77.59}); err != nil {
				t.Fatalf("other area search returned error: %v", err)
			}
			if len(rs.sent()) == before {
				t.Errorf("a different area was answered from the cache")
			}
		})
	}
}

func TestReporterLaneWaitsOnTheLimiterForEveryRequest(t *testing.T) {
	rs := newRecordingServer(t, nil)
	const every = 60 * time.Millisecond
	c := newClientWithBaseURL(rs.server.URL, "", every, 1)

	start := time.Now()
	if _, err := c.Search(context.Background(), longAddress, &Point{Lat: 17.4, Lon: 78.45}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	elapsed := time.Since(start)
	if n := len(rs.sent()); n != 4 {
		t.Fatalf("sent %d requests, want 4", n)
	}
	if elapsed < 3*every {
		t.Errorf("four requests took %v, want at least %v (one limiter token each)", elapsed, 3*every)
	}
}

func TestReaderLaneWaitsOnTheLimiterForEveryRequest(t *testing.T) {
	short2 := "Road No 12, Banjara Hills, Hyderabad"
	rs := newRecordingServer(t, func(q url.Values) bool { return q.Get("q") == short2 })
	const every = 60 * time.Millisecond
	c := newClientWithBaseURL(rs.server.URL, "", every, 1)

	start := time.Now()
	answer, err := c.SearchReader(context.Background(), longAddress, nil)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	elapsed := time.Since(start)
	if n := len(rs.sent()); n != 3 || answer.MatchedQuery != short2 {
		t.Fatalf("sent %d requests, matched %q; want 3 and %q", n, answer.MatchedQuery, short2)
	}
	if elapsed < 2*every {
		t.Errorf("three requests took %v, want at least %v (one limiter token each)", elapsed, 2*every)
	}
}

func TestRunningOutOfLimiterTimeIsBusyNotEmpty(t *testing.T) {
	t.Run("reporter", func(t *testing.T) {
		rs := newRecordingServer(t, nil)
		// The next token is further away than the bounded wait.
		c := newClientWithBaseURL(rs.server.URL, "", 10*time.Second, 1)
		answer, err := c.Search(context.Background(), longAddress, nil)
		if err == nil {
			t.Fatalf("Search returned %+v and no error, want a limiter error", answer)
		}
		if n := len(rs.sent()); n != 1 {
			t.Errorf("sent %d requests, want 1", n)
		}
		if n := c.reporter.size(); n != 0 {
			t.Errorf("reporter cache holds %d entries after a failure, want 0", n)
		}
	})
	t.Run("reader", func(t *testing.T) {
		rs := newRecordingServer(t, nil)
		c := newClientWithBaseURL(rs.server.URL, "", 10*time.Second, 1)
		start := time.Now()
		_, err := c.SearchReader(context.Background(), longAddress, nil)
		if !errors.Is(err, ErrReaderBusy) {
			t.Fatalf("SearchReader error = %v, want ErrReaderBusy", err)
		}
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("SearchReader took %v, want it to give up at once", elapsed)
		}
		if n := len(rs.sent()); n != 1 {
			t.Errorf("sent %d requests, want 1", n)
		}
		if n := c.reader.size(); n != 0 {
			t.Errorf("reader cache holds %d entries after a failure, want 0", n)
		}
	})
}

// TestReaderFollowUpNeverQueuesAheadOfAReporter is D-16 under the new
// design: a reader waiting between its own tries holds no reservation, so a
// reporter who starts in that gap is answered as if the reader were not
// there, and the reader gives up busy instead of sending its next try.
func TestReaderFollowUpNeverQueuesAheadOfAReporter(t *testing.T) {
	rs := newRecordingServer(t, nil)
	const every = 200 * time.Millisecond
	c := newClientWithBaseURL(rs.server.URL, "", every, 1)

	readerDone := make(chan error, 1)
	go func() {
		_, err := c.SearchReader(context.Background(), longAddress, nil)
		readerDone <- err
	}()

	// Let the reader send its first try and start waiting for its second.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if _, err := c.Search(context.Background(), "Bengaluru", nil); err != nil {
		t.Fatalf("reporter Search returned error: %v", err)
	}
	reporterTook := time.Since(start)
	// Without the reader the reporter waits about 150 ms; behind a reader's
	// reservation it would wait about 350 ms.
	if reporterTook > every+50*time.Millisecond {
		t.Errorf("reporter took %v, want under %v (it queued behind the reader)", reporterTook, every+50*time.Millisecond)
	}

	select {
	case err := <-readerDone:
		if !errors.Is(err, ErrReaderBusy) {
			t.Errorf("reader error = %v, want ErrReaderBusy", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reader search never returned")
	}
	if got, want := rs.texts(), []string{longAddress, "Bengaluru"}; !reflect.DeepEqual(got, want) {
		t.Errorf("texts sent = %q, want %q (the reader's next try is never sent)", got, want)
	}
}
