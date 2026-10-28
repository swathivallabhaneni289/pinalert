// demo_practice_e2e_test.go proves the Demo's practice posting (plan 05-17, D-01,
// D-05, T-05-91, T-05-92, T-05-95). A practice report is posted through the
// Demo API and climbs to Confirmed as the pretend neighbours vote through the
// real rules, with the server writing every sentence; nothing reaches a table.
// The Demo page's post window leaves out the Call 112 box and the What is
// happening picks (a tel link in a sandbox could dial real emergency services)
// and the address search, while the real Nearby page and the Low data view are
// unchanged.
//
// The first test needs a database; the rendering tests do not.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/api/handlers"
	"pinalert/internal/demo"
	"pinalert/internal/service"
)

// postPracticeMedium posts a medium flood practice report at the pretend
// position and returns its id.
func (d *demoE2E) postPracticeMedium() int64 {
	d.t.Helper()
	status, raw := d.do(http.MethodPost, "/reports", map[string]any{
		"category":    "flood",
		"severity":    "medium",
		"description": "Water over the road near the school",
		"latitude":    demo.PretendLat,
		"longitude":   demo.PretendLon,
	})
	if status != http.StatusCreated {
		d.t.Fatalf("POST Demo practice report = %d, want 201: %s", status, raw)
	}
	var out struct {
		Report struct {
			ID int64 `json:"id"`
		} `json:"report"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Report.ID == 0 {
		d.t.Fatalf("decoding the practice report answer %s: %v", raw, err)
	}
	return out.Report.ID
}

// practiceTrust reads the practice report's trust object and its Confirmed flag
// from a fresh Demo feed.
func (d *demoE2E) practiceTrust(id int64) (trust map[string]any, confirmed bool) {
	d.t.Helper()
	items, _ := d.feed()
	mine := itemByID(items, id)
	if mine == nil {
		d.t.Fatalf("the practice report %d is not in the Demo feed", id)
	}
	trust, _ = mine["trust"].(map[string]any)
	if trust == nil {
		d.t.Fatalf("the practice report %d has no trust object", id)
	}
	confirmed, _ = mine["confirmed_by_people"].(bool)
	return trust, confirmed
}

// TestDemoPracticeClimbOverHTTP posts a medium practice report through the Demo
// API and moves the world clock: the report starts not confirmed, a neighbour
// confirms it after the first interval and the second one after the next, and
// the server's own words change at each step. Every table keeps its row count.
func TestDemoPracticeClimbOverHTTP(t *testing.T) {
	d := newDemoE2E(t)
	before := tableCounts(t, d.pool)

	id := d.postPracticeMedium()

	// Step 0: nobody has confirmed. The server leads with what it needs.
	trust0, confirmed0 := d.practiceTrust(id)
	if confirmed0 {
		t.Fatalf("a new practice report reads confirmed before any neighbour voted")
	}
	if places, _ := trust0["confirmed_places"].(float64); places != 0 {
		t.Fatalf("confirmed_places = %v at the start, want 0", places)
	}
	lead0, _ := trust0["lead_sentence"].(string)
	if !strings.Contains(lead0, strconv.Itoa(service.IndependentAgreementThreshold)) {
		t.Fatalf("lead_sentence at the start = %q, want the server's threshold sentence naming %d people",
			lead0, service.IndependentAgreementThreshold)
	}

	// Step 1: the first neighbour has voted. Still not confirmed, and the server's
	// sentence changed to say how many more are needed.
	d.clock.Advance(demo.NeighbourInterval + time.Second)
	trust1, confirmed1 := d.practiceTrust(id)
	if confirmed1 {
		t.Fatalf("the practice report reads confirmed after one neighbour, threshold %d", service.IndependentAgreementThreshold)
	}
	if places, _ := trust1["confirmed_places"].(float64); places != 1 {
		t.Fatalf("confirmed_places after one interval = %v, want 1", places)
	}
	lead1, _ := trust1["lead_sentence"].(string)
	if lead1 == "" || lead1 == lead0 {
		t.Fatalf("lead_sentence after one neighbour = %q, want a new server sentence different from %q", lead1, lead0)
	}

	// Step 2: the second neighbour has voted. Confirmed, and the server's
	// confirmed sentence names the people at different spots.
	d.clock.Advance(demo.NeighbourInterval)
	trust2, confirmed2 := d.practiceTrust(id)
	if !confirmed2 {
		t.Fatalf("the practice report does not read confirmed after %d neighbours", service.IndependentAgreementThreshold)
	}
	if places, _ := trust2["confirmed_places"].(float64); int(places) != service.IndependentAgreementThreshold {
		t.Fatalf("confirmed_places when confirmed = %v, want %d: the neighbours stop at the real threshold",
			places, service.IndependentAgreementThreshold)
	}
	sentence, _ := trust2["confirmed_sentence"].(string)
	want := "Confirmed by " + strconv.Itoa(service.IndependentAgreementThreshold) + " people"
	if !strings.HasPrefix(sentence, want) {
		t.Fatalf("confirmed_sentence = %q, want the server's sentence starting %q", sentence, want)
	}

	// A later read changes nothing: the neighbours stopped.
	d.clock.Advance(30 * time.Second)
	trust3, confirmed3 := d.practiceTrust(id)
	if !confirmed3 || !reflect.DeepEqual(trust2["confirmed_sentence"], trust3["confirmed_sentence"]) {
		t.Fatalf("the practice report changed after the neighbours stopped: %v -> %v", trust2, trust3)
	}

	if got := tableCounts(t, d.pool); !reflect.DeepEqual(before, got) {
		t.Fatalf("a practice post and its climb changed the table row counts\nbefore: %v\nafter:  %v", before, got)
	}
}

// The three ids and the tel link the Demo's post window must not carry.
var demoFormForbidden = []string{`call-112-box`, `happening-fields`, `happening-group`, `tel:`, `Call 112`}

// TestDemoFormHasNoCallBoxOrPicks pins T-05-91: the Demo page, rendered by the
// real handler, holds no Call 112 box, no What is happening picks and no tel
// link, while the rest of the form is there (the Critical choice shows neither).
func TestDemoFormHasNoCallBoxOrPicks(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{})
	resp, body := demoPageGet(t, srv.Client(), srv.URL+"/demo")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /demo = %d, want 200", resp.StatusCode)
	}
	for _, bad := range demoFormForbidden {
		if strings.Contains(body, bad) {
			t.Errorf("the Demo page holds %q: a sandbox must not offer a real call", bad)
		}
	}
	for _, must := range []string{`id="severity-group"`, `data-severity="critical"`, `id="category-grid"`, `id="submit-report"`, `id="description"`} {
		if !strings.Contains(body, must) {
			t.Errorf("the Demo page lost %q from its post window", must)
		}
	}
}

// TestRealFormsKeepCallBoxAndPicks pins T-05-95: the real Nearby page keeps the
// Call 112 box and the What is happening picks exactly as before, and the Low
// data view keeps the form it was built with.
func TestRealFormsKeepCallBoxAndPicks(t *testing.T) {
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	cfg := handlers.PageConfig{FallbackLat: 12.9716, FallbackLon: 77.5946, DefaultRadiusKm: 10, VoteRadiusKm: 1, AssetVersion: "test"}
	signedIn := func(path string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		return req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 1, Email: "verified@example.com"}))
	}

	rec := httptest.NewRecorder()
	handlers.Page(tmpl, cfg)(rec, signedIn("/"))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	index := rec.Body.String()
	for _, must := range []string{`id="call-112-box"`, `id="happening-fields"`, `id="happening-group"`, `href="tel:`, `id="location-search"`, `id="location-notice"`} {
		if !strings.Contains(index, must) {
			t.Errorf("the real Nearby page lost %q", must)
		}
	}

	rec = httptest.NewRecorder()
	handlers.LowPage(tmpl, cfg)(rec, signedIn(handlers.LowDataPath))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", handlers.LowDataPath, rec.Code)
	}
	low := rec.Body.String()
	for _, must := range []string{`id="severity"`, `id="submit-report"`, `id="description"`} {
		if !strings.Contains(low, must) {
			t.Errorf("the Low data view lost %q", must)
		}
	}
	if strings.Contains(low, "data-demo") || strings.Contains(index, "data-demo") {
		t.Errorf("a real page carries the Demo flag")
	}
}

// TestDemoPostWindowHasNoSearch pins UI-SPEC 4d step 1: the Demo's post window
// has no address search and no location notice (the Demo never calls the
// geocoder and never asks for a location), and the shell names the pretend
// place for the line under the map.
func TestDemoPostWindowHasNoSearch(t *testing.T) {
	srv := demoPageServer(t, handlers.DemoLimit{})
	_, body := demoPageGet(t, srv.Client(), srv.URL+"/demo")
	for _, bad := range []string{`id="location-search"`, `id="location-search-results"`, `id="location-search-status"`, `id="location-notice"`, `location-search-field`} {
		if strings.Contains(body, bad) {
			t.Errorf("the Demo page still holds %q", bad)
		}
	}
	if !strings.Contains(body, `data-pretend-place="`+demo.PretendPlace+`"`) {
		t.Errorf("#app-shell does not carry data-pretend-place=%q", demo.PretendPlace)
	}
	if !strings.Contains(body, `id="near-place"`) {
		t.Errorf("the line under the map (#near-place) is gone")
	}
}
