// demo_test.go proves the Demo API's own rules without a database (plan 05-10):
// a bad visit is a 404, Demo writes take strict JSON, Flag and Mute do not
// exist in the Demo, and the write limit is per IP. The Demo reads and writes
// only the in memory world, so nothing here needs Postgres.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
)

// demoEndedSentence is the 404 sentence of a visit that is unknown or expired.
const demoEndedSentence = "This Demo has ended. Reload the page to start again."

// demoGenerousWrites keeps the write limit out of the way of the tests that are
// not about it.
var demoGenerousWrites = handlers.DemoLimit{Every: time.Millisecond, Burst: 10000}

// newDemoHandlerRouter mounts the Demo routes at /demo the way the real router
// does and returns the router with one seeded world's visit id.
func newDemoHandlerRouter(t *testing.T, write handlers.DemoLimit) (http.Handler, string) {
	t.Helper()
	store := demo.NewStore(demo.WithWorldSetup(demo.SeedSetup))
	w, err := store.Create()
	if err != nil {
		t.Fatalf("creating a Demo world: %v", err)
	}
	r := chi.NewRouter()
	r.Mount("/demo", handlers.DemoRoutes(handlers.DemoConfig{Store: store, WriteLimit: write}))
	return r, w.ID
}

// demoServe runs one request against h and returns the recorder.
func demoServe(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type demoErrBody struct {
	Error struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeDemoErr(t *testing.T, rec *httptest.ResponseRecorder) demoErrBody {
	t.Helper()
	var out demoErrBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding the error body %q: %v", rec.Body.String(), err)
	}
	return out
}

// demoNearbyPath is the feed read at the pretend position.
func demoNearbyPath(visit string) string {
	return "/demo/api/v/" + visit + "/reports?lat=16.5424&lon=80.6386&radius_km=10"
}

// demoReportCount reads how many reports the active view of the world holds.
func demoReportCount(t *testing.T, h http.Handler, visit string) int {
	t.Helper()
	rec := demoServe(h, http.MethodGet, demoNearbyPath(visit), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET Demo reports = %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Reports []json.RawMessage `json:"reports"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding the Demo feed: %v", err)
	}
	return len(out.Reports)
}

const demoPracticeBody = `{"category":"flood","severity":"low","description":"Water rising near the bridge","latitude":16.5424,"longitude":80.6386}`

func TestDemoRoutesRejectBadVisit(t *testing.T) {
	h, _ := newDemoHandlerRouter(t, demoGenerousWrites)
	paths := []string{
		"/demo/api/v/short/reports",
		"/demo/api/v/" + strings.Repeat("!", 22) + "/reports",
		"/demo/api/v/AAAAAAAAAAAAAAAAAAAAAA/reports", // well formed, unknown
	}
	for _, p := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			t.Run(method+" "+p, func(t *testing.T) {
				body := ""
				if method == http.MethodPost {
					body = demoPracticeBody
				}
				rec := demoServe(h, method, p+"?lat=16.5&lon=80.6", body)
				if rec.Code != http.StatusNotFound {
					t.Fatalf("%s %s = %d, want 404: %s", method, p, rec.Code, rec.Body.String())
				}
				e := decodeDemoErr(t, rec)
				if e.Error.Field != "visit" || e.Error.Message != demoEndedSentence {
					t.Fatalf("error = %+v, want field visit and the ended sentence", e.Error)
				}
				if got := rec.Header().Values("Set-Cookie"); len(got) != 0 {
					t.Fatalf("a Demo answer set a cookie: %v", got)
				}
			})
		}
	}
	// A vote on an unknown visit is the same 404.
	rec := demoServe(h, http.MethodPost, "/demo/api/v/AAAAAAAAAAAAAAAAAAAAAA/reports/1/confirm", `{"latitude":16.5424,"longitude":80.6386}`)
	if rec.Code != http.StatusNotFound || decodeDemoErr(t, rec).Error.Field != "visit" {
		t.Fatalf("vote on an unknown visit = %d %s, want 404 on field visit", rec.Code, rec.Body.String())
	}
}

func TestDemoStrictJSON(t *testing.T) {
	h, visit := newDemoHandlerRouter(t, demoGenerousWrites)
	before := demoReportCount(t, h, visit)
	reports := "/demo/api/v/" + visit + "/reports"

	big := `{"category":"flood","severity":"low","description":"` + strings.Repeat("a", 70*1024) + `","latitude":16.5424,"longitude":80.6386}`
	cases := []struct {
		name, path, body string
	}{
		{"unknown report field", reports, `{"category":"flood","severity":"low","description":"Water rising near the bridge","latitude":16.5424,"longitude":80.6386,"created_at":"2020-01-01T00:00:00Z"}`},
		{"report body over the limit", reports, big},
		{"report not JSON", reports, `this is not json`},
		{"report empty object", reports, `{}`},
		{"vote unknown field", reports + "/1/confirm", `{"latitude":16.5424,"longitude":80.6386,"geohash":"abc"}`},
		{"vote body over the limit", reports + "/1/confirm", `{"latitude":16.5424,"longitude":80.6386,"x":"` + strings.Repeat("b", 5*1024) + `"}`},
		{"vote not JSON", reports + "/1/confirm", `confirm please`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := demoServe(h, http.MethodPost, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %.200s", rec.Code, rec.Body.String())
			}
		})
	}
	if after := demoReportCount(t, h, visit); after != before {
		t.Fatalf("refused writes changed the report count from %d to %d, want no change", before, after)
	}

	// A good body still works, so the refusals above are not a dead route.
	rec := demoServe(h, http.MethodPost, reports, demoPracticeBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("a good practice report = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if after := demoReportCount(t, h, visit); after != before+1 {
		t.Fatalf("report count = %d, want %d after one good post", after, before+1)
	}
}

func TestDemoNoFlagOrMuteRoutes(t *testing.T) {
	h, visit := newDemoHandlerRouter(t, demoGenerousWrites)
	for _, action := range []string{"flag", "mute", "unmute"} {
		t.Run(action, func(t *testing.T) {
			rec := demoServe(h, http.MethodPost, "/demo/api/v/"+visit+"/reports/1/"+action, `{}`)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("POST %s = %d, want 404: the Demo has no such route", action, rec.Code)
			}
		})
	}
	// The five vote words that do exist are not 404 for a good body.
	for _, action := range []string{"confirm", "dispute", "resolve", "reopen", "unresolve"} {
		rec := demoServe(h, http.MethodPost, "/demo/api/v/"+visit+"/reports/1/"+action, `{"latitude":16.5424,"longitude":80.6386}`)
		if rec.Code == http.StatusNotFound && decodeDemoErr(t, rec).Error.Field == "" {
			t.Fatalf("POST %s is a plain 404: the route is missing", action)
		}
	}
}

func TestDemoRateLimitPerIP(t *testing.T) {
	h, visit := newDemoHandlerRouter(t, handlers.DemoLimit{Every: time.Hour, Burst: 2})
	post := func(remote string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/demo/api/v/"+visit+"/reports/1/confirm", bytes.NewBufferString(`{"latitude":16.5424,"longitude":80.6386}`))
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	for i := 1; i <= 2; i++ {
		if rec := post("192.0.2.10:4000"); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("write %d from one IP was limited, want it inside the burst of 2", i)
		}
	}
	rec := post("192.0.2.10:4001")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third write from one IP = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Too many requests. Try again in a minute.") {
		t.Fatalf("429 body = %s, want the standard message", rec.Body.String())
	}
	if rec := post("198.51.100.7:5000"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("another IP was limited by the first IP's writes")
	}
	// Reads are never limited.
	for i := 0; i < 5; i++ {
		if rec := demoServe(h, http.MethodGet, demoNearbyPath(visit), ""); rec.Code != http.StatusOK {
			t.Fatalf("read %d = %d, want 200 whatever the write limit", i, rec.Code)
		}
	}
}
