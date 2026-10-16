package geocode

// Tests for quick task 261016-sfu: when Photon finds nothing for the whole
// text, the search asks Nominatim once with the whole text, then Photon with
// the first and the second shorter text, inside the cap of four requests.
// Every upstream here is an httptest stand in; nothing calls the real Photon
// or Nominatim.

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

const (
	orderShort1 = "Sai Residency, Road No 12, Banjara Hills, Hyderabad"
	orderShort2 = "Road No 12, Banjara Hills, Hyderabad"
)

// orderStubs is a Photon stand in and a Nominatim stand in that write every
// request to one shared log, so a test can pin the order across both.
type orderStubs struct {
	mu        sync.Mutex
	log       []string
	nominatim []url.Values
	photon    *httptest.Server
	nom       *httptest.Server
}

// newOrderStubs starts both stand ins. photonFinds and nominatimFinds say
// which texts find something; photonFails says which texts Photon answers
// with a 500. A nil func finds (or fails) nothing.
func newOrderStubs(t *testing.T, photonFinds, nominatimFinds, photonFails func(text string) bool) *orderStubs {
	t.Helper()
	return newOrderStubsWithNominatimStatus(t, photonFinds, nominatimFinds, photonFails, http.StatusOK)
}

// newOrderStubsWithNominatimStatus is newOrderStubs with every Nominatim
// answer sent with the given status; anything but 200 carries no body.
func newOrderStubsWithNominatimStatus(t *testing.T, photonFinds, nominatimFinds, photonFails func(text string) bool, nominatimStatus int) *orderStubs {
	t.Helper()
	st := &orderStubs{}
	found := photonBody(t, photonFeatureJSON(78.4347, 17.4156, map[string]any{
		"name": "Road No 12", "district": "Banjara Hills", "city": "Hyderabad", "state": "Telangana",
		"countrycode": "IN", "osm_key": "highway",
	}))
	empty := photonBody(t)
	st.photon = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("q")
		st.mu.Lock()
		st.log = append(st.log, "photon: "+text)
		st.mu.Unlock()
		if photonFails != nil && photonFails(text) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if photonFinds != nil && photonFinds(text) {
			_, _ = w.Write(found)
			return
		}
		_, _ = w.Write(empty)
	}))
	t.Cleanup(st.photon.Close)
	st.nom = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		st.mu.Lock()
		st.log = append(st.log, "nominatim: "+q.Get("q"))
		st.nominatim = append(st.nominatim, q)
		st.mu.Unlock()
		if nominatimStatus != http.StatusOK {
			w.WriteHeader(nominatimStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if nominatimFinds != nil && nominatimFinds(q.Get("q")) {
			_, _ = w.Write([]byte(`[{"name":"Sai Residency","display_name":"Sai Residency, Road No 12, Banjara Hills, Hyderabad, Telangana, India","lat":"17.4156","lon":"78.4347"}]`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(st.nom.Close)
	return st
}

func (st *orderStubs) sent() []string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]string(nil), st.log...)
}

func (st *orderStubs) nominatimSent() []url.Values {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]url.Values(nil), st.nominatim...)
}

func (st *orderStubs) client() *Client {
	return newDualClient(st.photon.URL, st.nom.URL)
}

func is(want string) func(string) bool {
	return func(text string) bool { return text == want }
}

// TestOrderWhenPhotonFindsNothing pins the order and the cap on both lanes,
// with and without a position: Photon whole, Nominatim whole, Photon first
// shorter, Photon second shorter, then an empty non nil list.
func TestOrderWhenPhotonFindsNothing(t *testing.T) {
	for name, search := range lanes {
		for _, near := range []*Point{nil, {Lat: 17.4, Lon: 78.45}} {
			t.Run(name, func(t *testing.T) {
				st := newOrderStubs(t, nil, nil, nil)
				answer, err := search(st.client(), context.Background(), longAddress, near)
				if err != nil {
					t.Fatalf("search returned error: %v", err)
				}
				want := []string{
					"photon: " + longAddress,
					"nominatim: " + longAddress,
					"photon: " + orderShort1,
					"photon: " + orderShort2,
				}
				if got := st.sent(); !reflect.DeepEqual(got, want) {
					t.Fatalf("requests = %q, want %q", got, want)
				}
				if len(want) != maxUpstreamPerSearch {
					t.Fatalf("the order has %d requests, the cap is %d", len(want), maxUpstreamPerSearch)
				}
				if answer.Results == nil || len(answer.Results) != 0 || answer.MatchedQuery != "" {
					t.Fatalf("answer = %+v, want an empty non nil list and no matched text", answer)
				}
			})
		}
	}
}

// TestNominatimTryLooksNearWithoutABox: the one Nominatim try prefers places
// near the position (viewbox) but is not held inside it (no bounded), so a
// whole address in another city is still found. India only stays.
func TestNominatimTryLooksNearWithoutABox(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			st := newOrderStubs(t, nil, nil, nil)
			if _, err := search(st.client(), context.Background(), longAddress, &Point{Lat: 28.61, Lon: 77.21}); err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			sent := st.nominatimSent()
			if len(sent) != 1 {
				t.Fatalf("nominatim got %d requests, want 1", len(sent))
			}
			q := sent[0]
			if q.Get("viewbox") != "76.710000,29.110000,77.710000,28.110000" {
				t.Errorf("viewbox = %q, want the box around the position", q.Get("viewbox"))
			}
			if q.Has("bounded") {
				t.Errorf("bounded = %q, want it left out so places outside the box still count", q.Get("bounded"))
			}
			if q.Get("countrycodes") != "in" {
				t.Errorf("countrycodes = %q, want in", q.Get("countrycodes"))
			}

			// No position: no viewbox at all.
			st = newOrderStubs(t, nil, nil, nil)
			if _, err := search(st.client(), context.Background(), longAddress, nil); err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if q := st.nominatimSent()[0]; q.Has("viewbox") || q.Has("bounded") {
				t.Errorf("request without a position = %v, want no viewbox and no bounded", q)
			}
		})
	}
}

// TestOrderStopsAtTheFirstFind pins each place the order can stop, and the
// matched text it reports.
func TestOrderStopsAtTheFirstFind(t *testing.T) {
	cases := []struct {
		name           string
		photonFinds    func(string) bool
		nominatimFinds func(string) bool
		wantSent       []string
		wantMatched    string
		wantProvider   string
	}{
		{
			"nominatim finds the whole address",
			nil, is(longAddress),
			[]string{"photon: " + longAddress, "nominatim: " + longAddress},
			"", providerNominatim,
		},
		{
			"photon finds the first shorter text",
			is(orderShort1), nil,
			[]string{"photon: " + longAddress, "nominatim: " + longAddress, "photon: " + orderShort1},
			orderShort1, providerPhoton,
		},
		{
			// The Flat 4B case from the 261016-pho live check.
			"photon finds the second shorter text",
			is(orderShort2), nil,
			[]string{"photon: " + longAddress, "nominatim: " + longAddress, "photon: " + orderShort1, "photon: " + orderShort2},
			orderShort2, providerPhoton,
		},
	}
	for _, tc := range cases {
		for name, search := range lanes {
			t.Run(tc.name+" "+name, func(t *testing.T) {
				st := newOrderStubs(t, tc.photonFinds, tc.nominatimFinds, nil)
				answer, err := search(st.client(), context.Background(), longAddress, &Point{Lat: 17.4, Lon: 78.45})
				if err != nil {
					t.Fatalf("search returned error: %v", err)
				}
				if got := st.sent(); !reflect.DeepEqual(got, tc.wantSent) {
					t.Fatalf("requests = %q, want %q", got, tc.wantSent)
				}
				if len(answer.Results) != 1 || answer.MatchedQuery != tc.wantMatched || answer.provider != tc.wantProvider {
					t.Fatalf("answer = %+v, want one result, matched %q, from %s", answer, tc.wantMatched, tc.wantProvider)
				}
			})
		}
	}
}

// TestShortTextNeedsNoShorterTries: a text with no shorter form stops after
// Photon and Nominatim.
func TestShortTextNeedsNoShorterTries(t *testing.T) {
	st := newOrderStubs(t, nil, nil, nil)
	if _, err := st.client().Search(context.Background(), "Hyderabad", nil); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if got, want := st.sent(), []string{"photon: Hyderabad", "nominatim: Hyderabad"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requests = %q, want %q", got, want)
	}
}

// TestNominatimErrorInTheOrderEndsTheSearch: a Nominatim failure in the
// middle of the order is the "Search unavailable" error, never an empty list,
// and the shorter texts are not sent.
func TestNominatimErrorInTheOrderEndsTheSearch(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			st := newOrderStubsWithNominatimStatus(t, nil, nil, nil, http.StatusTooManyRequests)
			c := st.client()
			_, err := search(c, context.Background(), longAddress, nil)
			var status *UpstreamStatusError
			if !errors.As(err, &status) || status.Status != http.StatusTooManyRequests {
				t.Fatalf("error = %v, want the upstream status 429", err)
			}
			if got, want := st.sent(), []string{"photon: " + longAddress, "nominatim: " + longAddress}; !reflect.DeepEqual(got, want) {
				t.Fatalf("requests = %q, want %q", got, want)
			}
			if c.reporter.size() != 0 || c.reader.size() != 0 {
				t.Fatalf("an error was cached")
			}
		})
	}
}

// TestPhotonFailingOnAShorterTextHandsTheRestToNominatim: once Photon fails in
// a search it is not asked again; the texts left go to Nominatim, inside the
// cap, and an empty answer after the failure is not cached.
func TestPhotonFailingOnAShorterTextHandsTheRestToNominatim(t *testing.T) {
	for name, search := range lanes {
		t.Run(name+" found", func(t *testing.T) {
			st := newOrderStubs(t, nil, is(orderShort1), is(orderShort1))
			answer, err := search(st.client(), context.Background(), longAddress, nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			want := []string{
				"photon: " + longAddress,
				"nominatim: " + longAddress,
				"photon: " + orderShort1,
				"nominatim: " + orderShort1,
			}
			if got := st.sent(); !reflect.DeepEqual(got, want) {
				t.Fatalf("requests = %q, want %q", got, want)
			}
			if answer.MatchedQuery != orderShort1 || len(answer.Results) != 1 || answer.provider != providerNominatim {
				t.Fatalf("answer = %+v, want nominatim's answer matched on %q", answer, orderShort1)
			}
		})
		t.Run(name+" nothing", func(t *testing.T) {
			st := newOrderStubs(t, nil, nil, is(orderShort1))
			c := st.client()
			answer, err := search(c, context.Background(), longAddress, nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if got := st.sent(); len(got) != maxUpstreamPerSearch || got[3] != "nominatim: "+orderShort1 {
				t.Fatalf("requests = %q, want four ending with nominatim on the first shorter text", got)
			}
			if answer.Results == nil || len(answer.Results) != 0 {
				t.Fatalf("answer = %+v, want an empty list", answer)
			}
			if c.reporter.size() != 0 || c.reader.size() != 0 {
				t.Fatalf("an empty answer after a photon failure was cached")
			}
		})
	}
}

// TestOrderAnswerIsCachedUnderTheTypedText: the final answer, matched text
// included, is kept under what was typed, so typing it again sends nothing.
func TestOrderAnswerIsCachedUnderTheTypedText(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			st := newOrderStubs(t, is(orderShort2), nil, nil)
			c := st.client()
			near := &Point{Lat: 17.40, Lon: 78.45}
			if _, err := search(c, context.Background(), longAddress, near); err != nil {
				t.Fatalf("first search returned error: %v", err)
			}
			before := len(st.sent())
			again, err := search(c, context.Background(), "  flat 4B,  Sai Residency, Road No 12, Banjara Hills, HYDERABAD", &Point{Lat: 17.41, Lon: 78.46})
			if err != nil {
				t.Fatalf("repeat search returned error: %v", err)
			}
			if n := len(st.sent()) - before; n != 0 {
				t.Fatalf("repeat search sent %d requests, want 0", n)
			}
			if again.MatchedQuery != orderShort2 || len(again.Results) != 1 {
				t.Fatalf("cached answer = %+v, want the matched text kept", again)
			}
			// Typing the shorter text itself is answered from Photon's cache,
			// with no matched text, because that text matched as typed.
			short, err := search(c, context.Background(), orderShort2, near)
			if err != nil {
				t.Fatalf("shorter text search returned error: %v", err)
			}
			if n := len(st.sent()) - before; n != 0 || short.MatchedQuery != "" || len(short.Results) != 1 {
				t.Fatalf("shorter text search sent %d, answer %+v; want 0 and an exact match", n, short)
			}
		})
	}
}

// TestReporterWaitsLongerForALaterPhotonToken: the first Photon try waits at
// most photonLimiterWait before falling back, but a shorter try waits up to
// limiterWaitTimeout, so at one Photon request every 0.7 s the Flat 4B
// address still reaches its second shorter text.
func TestReporterWaitsLongerForALaterPhotonToken(t *testing.T) {
	st := newOrderStubs(t, is(orderShort2), nil, nil)
	c := newClientWithProviders(st.photon.URL, st.nom.URL, "", time.Millisecond, 10, 700*time.Millisecond, 1)
	answer, err := c.Search(context.Background(), longAddress, nil)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if answer.MatchedQuery != orderShort2 || answer.provider != providerPhoton {
		t.Fatalf("answer = %+v, want photon's answer matched on %q", answer, orderShort2)
	}
	if n := len(st.sent()); n != 4 {
		t.Fatalf("sent %d requests, want 4", n)
	}
}

// TestReaderShorterPhotonTryNeverQueuesAheadOfAReporter is D-16 between the
// reader's Photon tries: a reader waiting for its next Photon token holds no
// reservation, so a reporter who starts in that gap gets the token and the
// reader gives up busy without sending its shorter text.
func TestReaderShorterPhotonTryNeverQueuesAheadOfAReporter(t *testing.T) {
	st := newOrderStubs(t, nil, nil, nil)
	const every = 300 * time.Millisecond
	c := newClientWithProviders(st.photon.URL, st.nom.URL, "", time.Millisecond, 10, every, 1)

	readerDone := make(chan error, 1)
	go func() {
		_, err := c.SearchReader(context.Background(), longAddress, nil)
		readerDone <- err
	}()
	// The reader sends Photon and Nominatim at once, then waits about 300 ms
	// for Photon's next token.
	time.Sleep(80 * time.Millisecond)
	start := time.Now()
	if _, err := c.Search(context.Background(), "Bengaluru", nil); err != nil {
		t.Fatalf("reporter Search returned error: %v", err)
	}
	if took := time.Since(start); took > every+100*time.Millisecond {
		t.Errorf("reporter took %v, want under %v (it queued behind the reader)", took, every+100*time.Millisecond)
	}
	select {
	case err := <-readerDone:
		if !errors.Is(err, ErrReaderBusy) {
			t.Fatalf("reader error = %v, want ErrReaderBusy", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reader search never returned")
	}
	for _, line := range st.sent() {
		if line == "photon: "+orderShort1 {
			t.Fatalf("the reader's shorter text was sent: %q", st.sent())
		}
	}
}
