// Package handlers_test also covers OPS-01's browsable API reference. It
// lives outside package handlers (like reports_e2e_test.go) because it
// needs to import internal/api, which imports internal/api/handlers itself.
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pinalert/internal/api"
	"pinalert/internal/service"
	"pinalert/internal/session"
	sqlcgen "pinalert/internal/store/sqlc"
)

// fakeDBTX satisfies sqlcgen.DBTX without a real database connection.
// TestSwaggerDocServed drives the real router end to end, and the session
// middleware's persist callback (UpsertSession) runs on every request —
// including a request to /swagger/*, since it's registered via r.Use, not
// scoped to the /api group. This test never exercises /api/reports itself,
// so a no-op Exec is all the persist callback needs to succeed without a
// real Postgres, keeping this test runnable under `-short`.
type fakeDBTX struct{}

func (fakeDBTX) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (fakeDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, errNotSupported
}

func (fakeDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

var errNotSupported = &notSupportedError{}

type notSupportedError struct{}

func (*notSupportedError) Error() string {
	return "fakeDBTX: Query is not supported; this test only needs Exec for session persistence"
}

// newSwaggerTestRouter builds a real api.NewRouter router with no real
// database behind it — sufficient for exercising /swagger/* and the
// session middleware that wraps every route, but not for exercising
// /api/reports itself.
func newSwaggerTestRouter(t *testing.T) http.Handler {
	t.Helper()

	mgr, err := session.NewManager([]byte("swagger-test-secret-not-for-production"), false)
	if err != nil {
		t.Fatalf("constructing session manager: %v", err)
	}

	deps := api.Deps{
		Session:  mgr,
		Sessions: sqlcgen.New(fakeDBTX{}),
		Reports:  service.NewReportService(nil),
	}
	return api.NewRouter(deps)
}

// TestSwaggerDocServed drives the real router through httptest and asserts
// that OPS-01's stable URL actually serves both the raw spec and the
// browsable UI. Named per 01-VALIDATION.md's verification map.
func TestSwaggerDocServed(t *testing.T) {
	srv := httptest.NewServer(newSwaggerTestRouter(t))
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/swagger/doc.json")
	if err != nil {
		t.Fatalf("GET /swagger/doc.json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /swagger/doc.json status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("decoding /swagger/doc.json as JSON: %v", err)
	}
	_, hasSwagger := spec["swagger"]
	_, hasOpenAPI := spec["openapi"]
	if !hasSwagger && !hasOpenAPI {
		t.Fatalf("/swagger/doc.json has neither a \"swagger\" nor an \"openapi\" version key: %v", spec)
	}

	resp2, err := http.Get(srv.URL + "/swagger/index.html")
	if err != nil {
		t.Fatalf("GET /swagger/index.html: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /swagger/index.html status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}
	if ct := resp2.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("GET /swagger/index.html Content-Type = %q, want it to contain %q", ct, "text/html")
	}
}

// p07GuardedSwaggerRoots are the definitions whose property names the trust
// guard checks, together with every definition they reference. Plan 03-14
// appended handlers.CurrencyResponse and handlers.ReporterResponse here. The
// auth definitions, which carry email by design, are never roots.
//
// Plan 03.1-10 appended handlers.ReportListResponse, which reaches
// HistoryResponse, RecentResponse, CutResponse and FocusResponse, so the new
// wire objects of the history views are held to the same name and nullable
// rules.
//
// Plan 04-15 appended handlers.RateLimitResponse, the one 429 body.
var p07GuardedSwaggerRoots = []string{"handlers.TrustResponse", "handlers.FeedReportResponse", "handlers.CastVoteResponse", "handlers.CurrencyResponse", "handlers.ReporterResponse", "handlers.ReportListResponse", "handlers.RateLimitResponse"}

// p15Assert429 fails unless the operation documents a 429 response that
// references handlers.RateLimitResponse and declares a Retry-After header.
func p15Assert429(t *testing.T, label string, op json.RawMessage) {
	t.Helper()
	var parsed struct {
		Responses map[string]struct {
			Schema struct {
				Ref string `json:"$ref"`
			} `json:"schema"`
			Headers map[string]json.RawMessage `json:"headers"`
		} `json:"responses"`
	}
	if err := json.Unmarshal(op, &parsed); err != nil {
		t.Errorf("%s: parsing the operation: %v", label, err)
		return
	}
	resp, ok := parsed.Responses["429"]
	if !ok {
		t.Errorf("%s: no 429 response", label)
		return
	}
	if resp.Schema.Ref != "#/definitions/handlers.RateLimitResponse" {
		t.Errorf("%s: 429 schema ref = %q, want #/definitions/handlers.RateLimitResponse", label, resp.Schema.Ref)
	}
	if _, ok := resp.Headers["Retry-After"]; !ok {
		t.Errorf("%s: 429 response has no Retry-After header", label)
	}
}

// p07IdentifierPropertyName is the name pattern no guarded property may match.
var p07IdentifierPropertyName = regexp.MustCompile(`(?i)cell|account|session|email`)

// p07SwaggerRefs walks a decoded JSON value and records the definition name
// of every "$ref" that points at #/definitions/, wherever it sits (a bare
// ref, an allOf wrapper, an array items entry).
func p07SwaggerRefs(v any, out map[string]bool) {
	const prefix = "#/definitions/"
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if s, ok := val.(string); ok && k == "$ref" && strings.HasPrefix(s, prefix) {
				out[strings.TrimPrefix(s, prefix)] = true
				continue
			}
			p07SwaggerRefs(val, out)
		}
	case []any:
		for _, val := range x {
			p07SwaggerRefs(val, out)
		}
	}
}

// p07WalkSwaggerDefinitions works breadth first from roots, following every
// reference found inside a property value, and returns the visited
// definitions with their decoded properties. A missing root is fatal.
func p07WalkSwaggerDefinitions(t *testing.T, defs map[string]json.RawMessage, roots []string) (visited map[string]bool, props map[string]map[string]json.RawMessage) {
	t.Helper()
	visited = map[string]bool{}
	props = map[string]map[string]json.RawMessage{}
	queue := append([]string(nil), roots...)
	for _, r := range roots {
		if _, ok := defs[r]; !ok {
			t.Fatalf("docs/swagger.json is missing the guarded definition %q", r)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if visited[name] {
			continue
		}
		raw, ok := defs[name]
		if !ok {
			continue
		}
		visited[name] = true
		var def struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &def); err != nil {
			t.Fatalf("parsing definition %q: %v", name, err)
		}
		props[name] = def.Properties
		for _, pv := range def.Properties {
			var decoded any
			if err := json.Unmarshal(pv, &decoded); err != nil {
				t.Fatalf("parsing a property of %q: %v", name, err)
			}
			refs := map[string]bool{}
			p07SwaggerRefs(decoded, refs)
			for ref := range refs {
				if _, exists := defs[ref]; exists && !visited[ref] {
					queue = append(queue, ref)
				}
			}
		}
	}
	return visited, props
}

// p14SwaggerProp returns one decoded property of a walked definition and fails
// when the definition or the property is missing.
func p14SwaggerProp(t *testing.T, props map[string]map[string]json.RawMessage, def, key string) map[string]any {
	t.Helper()
	ps, ok := props[def]
	if !ok {
		t.Fatalf("the guard walk never visited %s", def)
	}
	raw, ok := ps[key]
	if !ok {
		t.Fatalf("%s is missing property %q", def, key)
	}
	var prop map[string]any
	if err := json.Unmarshal(raw, &prop); err != nil {
		t.Fatalf("parsing %s property %q: %v", def, key, err)
	}
	return prop
}

// p14AssertPropKeys fails unless the definition has exactly the wanted
// property names.
func p14AssertPropKeys(t *testing.T, props map[string]map[string]json.RawMessage, def string, want []string) {
	t.Helper()
	ps, ok := props[def]
	if !ok {
		t.Fatalf("the guard walk never visited %s", def)
	}
	wantSet := map[string]bool{}
	for _, k := range want {
		wantSet[k] = true
		if _, ok := ps[k]; !ok {
			t.Errorf("%s is missing property %q", def, k)
		}
	}
	for k := range ps {
		if !wantSet[k] {
			t.Errorf("%s has unexpected property %q", def, k)
		}
	}
}

// p14AssertEnum fails unless the property's enum equals want element by
// element, in order.
func p14AssertEnum(t *testing.T, prop map[string]any, want []string) {
	t.Helper()
	raw, ok := prop["enum"].([]any)
	if !ok {
		t.Fatalf("property has no enum array: %#v", prop)
	}
	got := make([]string, 0, len(raw))
	for _, v := range raw {
		s, _ := v.(string)
		got = append(got, s)
	}
	if len(got) != len(want) {
		t.Fatalf("enum = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("enum[%d] = %q, want %q (enum %v)", i, got[i], want[i], got)
		}
	}
}

// TestSwaggerSpecCoversRoutes is the drift guard: it loads the committed
// docs/swagger.json directly (not the live /swagger/doc.json response) so
// it fails the moment `make swag` is forgotten after a handler shape
// changes, and it is the published-schema counterpart to the runtime
// session_id leak guard in reports_e2e_test.go's
// TestNearbyResponseOmitsSessionID (threat register T-01-02/T-01-26).
func TestSwaggerSpecCoversRoutes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("reading docs/swagger.json: %v", err)
	}

	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parsing docs/swagger.json: %v", err)
	}

	reportsOps, ok := spec.Paths["/reports"]
	if !ok {
		t.Fatalf("docs/swagger.json is missing the /reports path")
	}
	for _, method := range []string{"get", "post"} {
		if _, ok := reportsOps[method]; !ok {
			t.Errorf("docs/swagger.json is missing the /reports %s operation", method)
		}
	}

	// 01.1-04's drift guard: both /reports operations must declare the 401
	// an unverified caller now receives from the access gate, not just the
	// 200/400 shapes that predate it.
	for _, method := range []string{"get", "post"} {
		op, ok := reportsOps[method]
		if !ok {
			continue // already reported above
		}
		if !strings.Contains(string(op), `"401"`) {
			t.Errorf("docs/swagger.json /reports %s operation is missing a 401 response", method)
		}
	}

	// 01.1-04: the request-link endpoint that makes a session verified in
	// the first place must itself be documented.
	if _, ok := spec.Paths["/auth/request-link"]; !ok {
		t.Errorf("docs/swagger.json is missing the /auth/request-link path")
	}

	// 02-03b's contribution to this same OPS-01 drift guard: four vote
	// routes shipped, four vote routes documented, each with its 401 and
	// 403 response declared — a mounted route that ships without a
	// regenerated spec fails here rather than silently drifting.
	for _, path := range []string{
		"/reports/{id}/confirm", "/reports/{id}/dispute",
		"/reports/{id}/resolve", "/reports/{id}/reopen",
	} {
		ops, ok := spec.Paths[path]
		if !ok {
			t.Errorf("docs/swagger.json is missing the %s path", path)
			continue
		}
		op, ok := ops["post"]
		if !ok {
			t.Errorf("docs/swagger.json is missing the %s post operation", path)
			continue
		}
		for _, code := range []string{`"401"`, `"403"`} {
			if !strings.Contains(string(op), code) {
				t.Errorf("docs/swagger.json %s post operation is missing a %s response", path, code)
			}
		}
	}

	// 03.1-10: the fifth vote route, the take back of a mistaken resolve,
	// with the same 401 and 403 and the 409 a refused take back answers.
	unresolveOps, ok := spec.Paths["/reports/{id}/unresolve"]
	if !ok {
		t.Errorf("docs/swagger.json is missing the /reports/{id}/unresolve path")
	} else if op, ok := unresolveOps["post"]; !ok {
		t.Errorf("docs/swagger.json is missing the /reports/{id}/unresolve post operation")
	} else {
		for _, code := range []string{`"401"`, `"403"`, `"409"`} {
			if !strings.Contains(string(op), code) {
				t.Errorf("docs/swagger.json /reports/{id}/unresolve post operation is missing a %s response", code)
			}
		}
	}

	// 04-15's contribution: every 429 Phase 4 wires so far is documented, on
	// POST /reports and on each of the five vote routes.
	if op, ok := reportsOps["post"]; ok {
		p15Assert429(t, "POST /reports", op)
	}
	for _, path := range []string{
		"/reports/{id}/confirm", "/reports/{id}/dispute",
		"/reports/{id}/resolve", "/reports/{id}/reopen",
		"/reports/{id}/unresolve",
	} {
		if op, ok := spec.Paths[path]["post"]; ok {
			p15Assert429(t, "POST "+path, op)
		}
	}

	// 02-04's contribution to this same OPS-01 drift guard: one response
	// shape changed (GET /reports now returns FeedReportResponse, not
	// ReportResponse), one spec regenerated. The GET /reports operation
	// must declare the show_disputed parameter. 03.1-10 adds the view,
	// category and report parameters, all as query parameters.
	if getOp, ok := reportsOps["get"]; ok {
		var op struct {
			Parameters []struct {
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"parameters"`
		}
		if err := json.Unmarshal(getOp, &op); err != nil {
			t.Fatalf("parsing the GET /reports operation: %v", err)
		}
		declared := map[string]string{}
		for _, p := range op.Parameters {
			declared[p.Name] = p.In
		}
		for _, name := range []string{"lat", "lon", "radius_km", "view", "show_disputed", "category", "report"} {
			if in, ok := declared[name]; !ok {
				t.Errorf("docs/swagger.json GET /reports operation is missing the %s parameter", name)
			} else if in != "query" {
				t.Errorf("docs/swagger.json GET /reports parameter %s is in %q, want query", name, in)
			}
		}
	}

	body := string(raw)

	// 02-04: the GET /reports element shape and its four new response
	// fields must be documented so 02-06's display-copy mapping has a
	// contract to key off.
	if !strings.Contains(body, "FeedReportResponse") {
		t.Errorf("docs/swagger.json is missing the FeedReportResponse definition")
	}
	for _, field := range []string{"visibility_reason", "your_vote", "is_own_report"} {
		if !strings.Contains(body, field) {
			t.Errorf("docs/swagger.json is missing response field %q", field)
		}
	}

	for _, category := range []string{
		"flood", "earthquake", "fire", "storm_cyclone", "road_blocked",
		"power_outage", "shelter_open", "traffic", "rescue_needed", "other",
	} {
		if !strings.Contains(body, category) {
			t.Errorf("docs/swagger.json is missing category enum value %q", category)
		}
	}
	for _, severity := range []string{"low", "medium", "critical"} {
		if !strings.Contains(body, severity) {
			t.Errorf("docs/swagger.json is missing severity enum value %q", severity)
		}
	}
	// 02-03b: the four visibility enum values must be published so 02-06's
	// display-copy mapping has a documented contract to key off.
	for _, visibility := range []string{"hidden", "provisional", "live", "retracted"} {
		if !strings.Contains(body, visibility) {
			t.Errorf("docs/swagger.json is missing visibility enum value %q", visibility)
		}
	}

	// 07-03's contribution to this same OPS-01 drift guard: GET /geocode
	// (plan 07-01/07-03) must be published with its 401 (the access gate)
	// and 503 (D-04's unavailable path) responses, its q query parameter,
	// and the GeocodeResponse shape.
	geocodeOps, ok := spec.Paths["/geocode"]
	if !ok {
		t.Fatalf("docs/swagger.json is missing the /geocode path")
	}
	geocodeOp, ok := geocodeOps["get"]
	if !ok {
		t.Fatalf("docs/swagger.json is missing the /geocode get operation")
	}
	geocodeOpRaw := string(geocodeOp)
	for _, code := range []string{`"401"`, `"503"`} {
		if !strings.Contains(geocodeOpRaw, code) {
			t.Errorf("docs/swagger.json /geocode get operation is missing a %s response", code)
		}
	}
	if !strings.Contains(geocodeOpRaw, `"q"`) {
		t.Errorf("docs/swagger.json /geocode get operation is missing the q query parameter")
	}
	if !strings.Contains(body, "GeocodeResponse") {
		t.Errorf("docs/swagger.json is missing the GeocodeResponse definition")
	}
	for _, field := range []string{"display_name", "lat", "lon"} {
		if !strings.Contains(body, field) {
			t.Errorf("docs/swagger.json is missing GeocodeResult field %q", field)
		}
	}
	// 03.1-10: GET /geocode/place (plan 03.1-03) must be published with its q
	// parameter and the 200, 400, 401, 429 and 503 answers it gives.
	placeOps, ok := spec.Paths["/geocode/place"]
	if !ok {
		t.Fatalf("docs/swagger.json is missing the /geocode/place path")
	}
	placeOp, ok := placeOps["get"]
	if !ok {
		t.Fatalf("docs/swagger.json is missing the /geocode/place get operation")
	}
	placeOpRaw := string(placeOp)
	for _, code := range []string{`"200"`, `"400"`, `"401"`, `"429"`, `"503"`} {
		if !strings.Contains(placeOpRaw, code) {
			t.Errorf("docs/swagger.json /geocode/place get operation is missing a %s response", code)
		}
	}
	if !strings.Contains(placeOpRaw, `"q"`) {
		t.Errorf("docs/swagger.json /geocode/place get operation is missing the q query parameter")
	}

	// Published-schema counterpart to T-07-04: a documented field is an
	// invitation for a future change to start populating it, which is
	// exactly why this allowlist is asserted in the published schema and
	// not only at runtime (internal/api/handlers/geocode_test.go's
	// TestGeocode_AllowlistsResponseFields).
	for _, field := range []string{"place_id", "osm_id", "place_rank", "boundingbox"} {
		if strings.Contains(body, field) {
			t.Fatalf("docs/swagger.json must never document Nominatim-internal field %q", field)
		}
	}

	// 03-07: the trust object (V-30). The definition must carry the seven
	// wire keys, and the two response shapes that return it must reference
	// it. The name guard walks only the guarded roots and what they
	// reference, so the auth definitions that carry email by design are
	// never visited.
	var defSpec struct {
		Definitions map[string]json.RawMessage `json:"definitions"`
	}
	if err := json.Unmarshal(raw, &defSpec); err != nil {
		t.Fatalf("parsing docs/swagger.json definitions: %v", err)
	}
	visited, defProps := p07WalkSwaggerDefinitions(t, defSpec.Definitions, p07GuardedSwaggerRoots)
	trustProps := defProps["handlers.TrustResponse"]
	for _, key := range p07TrustWireKeys {
		if _, ok := trustProps[key]; !ok {
			t.Errorf("handlers.TrustResponse is missing property %q", key)
		}
	}
	if len(trustProps) != len(p07TrustWireKeys) {
		t.Errorf("handlers.TrustResponse has %d properties, want %d", len(trustProps), len(p07TrustWireKeys))
	}
	// 03-14: the types are named explicitly. The two signal objects are
	// checked as references, because they have no scalar type.
	for key, wantType := range map[string]string{
		"confirmed_places":   "integer",
		"disputed_places":    "integer",
		"confirmed_text":     "string",
		"disputed_text":      "string",
		"lead_sentence":      "string",
		"confirmed_sentence": "string",
		"disputed_sentence":  "string",
	} {
		prop := p14SwaggerProp(t, defProps, "handlers.TrustResponse", key)
		if got, _ := prop["type"].(string); got != wantType {
			t.Errorf("handlers.TrustResponse property %q has type %q, want %q", key, got, wantType)
		}
	}
	for key, want := range map[string]struct{ ref, words string }{
		"currency": {"handlers.CurrencyResponse", "still current signal"},
		"reporter": {"handlers.ReporterResponse", "reporter signal"},
	} {
		prop := p14SwaggerProp(t, defProps, "handlers.TrustResponse", key)
		refs := map[string]bool{}
		p07SwaggerRefs(prop, refs)
		if !refs[want.ref] {
			t.Errorf("handlers.TrustResponse property %q does not reference %s", key, want.ref)
		}
		if desc, _ := prop["description"].(string); !strings.Contains(desc, want.words) {
			t.Errorf("handlers.TrustResponse property %q has description %q, want it to contain %q", key, desc, want.words)
		}
	}

	// 03-14 (V-30 extension): the two signal definitions, their exact key
	// sets, their enums tied element by element to the service lists, and
	// x-nullable as the boolean true on exactly the three nullable properties.
	// 04-15: the one 429 body, and the held field of a created report (04-11).
	p14AssertPropKeys(t, defProps, "handlers.RateLimitResponse", []string{"emergency_number", "error", "retry_after_seconds", "serious"})
	var reportDef struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(defSpec.Definitions["handlers.ReportResponse"], &reportDef); err != nil {
		t.Fatalf("parsing handlers.ReportResponse: %v", err)
	}
	if _, ok := reportDef.Properties["held"]; !ok {
		t.Errorf("handlers.ReportResponse is missing the held property")
	}
	p14AssertPropKeys(t, defProps, "handlers.CurrencyResponse", p14CurrencyWireKeys)
	p14AssertPropKeys(t, defProps, "handlers.ReporterResponse", p14ReporterWireKeys)

	// 03.1-10: the wire shape of the history views. Exact key sets and the
	// types of the new objects, so a renamed or added property fails here.
	p14AssertPropKeys(t, defProps, "handlers.HistoryResponse",
		[]string{"state", "at", "closed_by", "closed_text", "outcome", "outcome_label", "lifetime_text"})
	p14AssertPropKeys(t, defProps, "handlers.RecentResponse", []string{"resolved_24h", "expired_24h"})
	p14AssertPropKeys(t, defProps, "handlers.CutResponse", []string{"resolved", "expired", "resolved_partial", "expired_partial"})
	p14AssertPropKeys(t, defProps, "handlers.FocusResponse", []string{"status", "report"})
	p14AssertPropKeys(t, defProps, "handlers.ReportListResponse", []string{"view", "reports", "recent", "cut", "focus"})
	for def, types := range map[string]map[string]string{
		"handlers.HistoryResponse": {
			"state": "string", "at": "string", "closed_by": "string", "closed_text": "string",
			"outcome": "string", "outcome_label": "string", "lifetime_text": "string",
		},
		"handlers.RecentResponse": {"resolved_24h": "integer", "expired_24h": "integer"},
		"handlers.CutResponse": {
			"resolved": "boolean", "expired": "boolean", "resolved_partial": "boolean", "expired_partial": "boolean",
		},
		"handlers.FocusResponse": {"status": "string"},
		"handlers.FeedReportResponse": {
			"view": "string", "approximate": "boolean", "your_resolution": "string", "undos_left": "integer",
		},
		"handlers.CastVoteResponse":   {"undos_left": "integer"},
		"handlers.ReportListResponse": {"view": "string", "reports": "array"},
	} {
		for key, wantType := range types {
			prop := p14SwaggerProp(t, defProps, def, key)
			if got, _ := prop["type"].(string); got != wantType {
				t.Errorf("%s property %q has type %q, want %q", def, key, got, wantType)
			}
		}
	}
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.HistoryResponse", "state"), []string{"resolved", "expired"})
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.FocusResponse", "status"), []string{"ok", "too_old", "unavailable"})
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.FeedReportResponse", "view"), []string{"active", "disputed", "resolved", "past"})
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.ReportListResponse", "view"), []string{"all", "active", "disputed", "past"})
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.FeedReportResponse", "your_resolution"), []string{"resolve", "reopen", "unresolve"})
	for owner, keys := range map[string]map[string]string{
		"handlers.FeedReportResponse": {"history": "handlers.HistoryResponse"},
		"handlers.ReportListResponse": {
			"recent": "handlers.RecentResponse", "cut": "handlers.CutResponse", "focus": "handlers.FocusResponse",
		},
		"handlers.FocusResponse": {"report": "handlers.FeedReportResponse"},
	} {
		for key, wantRef := range keys {
			refs := map[string]bool{}
			p07SwaggerRefs(p14SwaggerProp(t, defProps, owner, key), refs)
			if !refs[wantRef] {
				t.Errorf("%s property %q does not reference %s", owner, key, wantRef)
			}
		}
	}

	var bands, stages, tags []string
	for _, b := range service.CurrencyBands {
		bands = append(bands, string(b))
	}
	for _, s := range service.FadeStages {
		stages = append(stages, string(s))
	}
	for _, g := range service.ReporterTags {
		tags = append(tags, string(g))
	}
	nullable := map[string]bool{
		"handlers.CurrencyResponse.score":      true,
		"handlers.CurrencyResponse.fade_stage": true,
		"handlers.ReporterResponse.score":      true,
	}
	signalTypes := map[string]map[string]string{
		"handlers.CurrencyResponse": {
			"band": "string", "score": "integer", "label": "string", "sentence": "string",
			"score_sentence": "string", "recent_places": "integer", "window_minutes": "integer", "fade_stage": "string",
		},
		"handlers.ReporterResponse": {
			"tag": "string", "score": "integer", "label": "string", "sentence": "string",
			"score_sentence": "string", "judged_reports": "integer", "corroborated_reports": "integer",
		},
	}
	for def, types := range signalTypes {
		for key, wantType := range types {
			prop := p14SwaggerProp(t, defProps, def, key)
			if got, _ := prop["type"].(string); got != wantType {
				t.Errorf("%s property %q has type %q, want %q", def, key, got, wantType)
			}
		}
	}
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.CurrencyResponse", "band"), bands)
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.CurrencyResponse", "fade_stage"), stages)
	p14AssertEnum(t, p14SwaggerProp(t, defProps, "handlers.ReporterResponse", "tag"), tags)
	// x-nullable must be the boolean true on the three nullable properties and
	// absent from every other property of the five guarded definitions.
	for def, ps := range defProps {
		for key := range ps {
			prop := p14SwaggerProp(t, defProps, def, key)
			v, present := prop["x-nullable"]
			if nullable[def+"."+key] {
				if b, isBool := v.(bool); !present || !isBool || !b {
					t.Errorf("%s property %q must carry x-nullable as the boolean true, got %#v", def, key, v)
				}
			} else if present {
				t.Errorf("%s property %q carries x-nullable, only the three nullable values may", def, key)
			}
		}
	}
	for _, owner := range []string{"handlers.FeedReportResponse", "handlers.CastVoteResponse"} {
		pv, ok := defProps[owner]["trust"]
		if !ok {
			t.Errorf("%s is missing the trust property", owner)
			continue
		}
		var decoded any
		if err := json.Unmarshal(pv, &decoded); err != nil {
			t.Fatalf("parsing %s trust property: %v", owner, err)
		}
		refs := map[string]bool{}
		p07SwaggerRefs(decoded, refs)
		if !refs["handlers.TrustResponse"] {
			t.Errorf("%s trust property does not reference handlers.TrustResponse", owner)
		}
	}
	for _, root := range p07GuardedSwaggerRoots {
		if !visited[root] {
			t.Errorf("guard walk never visited %s", root)
		}
	}
	for _, auth := range []string{"handlers.RequestLinkRequest", "handlers.RequestLinkResponse"} {
		if visited[auth] {
			t.Errorf("guard walk visited %s, so it is not scoped to the response definitions", auth)
		}
	}
	for def, ps := range defProps {
		for key := range ps {
			if p07IdentifierPropertyName.MatchString(key) {
				t.Errorf("guarded definition %s has property %q, a name that looks like an identifier", def, key)
			}
		}
	}

	// 03.2-07: the five update and mute routes are documented, each with its
	// method and the 401 an unverified caller receives.
	for _, want := range []struct{ path, method string }{
		{"/updates", "get"},
		{"/updates/read", "post"},
		{"/reports/{id}/mute", "post"},
		{"/reports/{id}/unmute", "post"},
		{"/me/muted-reports", "get"},
		{"/me/delete", "post"},
		{"/me/logout-all", "post"},
		{"/me/alerts", "patch"},
	} {
		op, ok := spec.Paths[want.path][want.method]
		if !ok {
			t.Errorf("docs/swagger.json is missing the %s %s operation", want.path, want.method)
			continue
		}
		if !strings.Contains(string(op), `"401"`) {
			t.Errorf("docs/swagger.json %s %s operation is missing a 401 response", want.path, want.method)
		}
	}

	// The published-schema counterpart to the runtime information-disclosure
	// control (T-01-02): a session identifier must never be advertised as
	// part of the contract, because a documented field is an invitation for
	// a future change to start populating it.
	if strings.Contains(body, "session_id") || strings.Contains(body, "sessionId") {
		t.Fatalf("docs/swagger.json must never document a session_id/sessionId field")
	}

	// 02-04's own contribution to the same rule: the reporter account id
	// the feed loads to compute is_own_report must reduce to a boolean on
	// the wire and never travel as a value (T-01-02's published-schema
	// counterpart for this plan's new field).
	if strings.Contains(body, "reporter_account_id") || strings.Contains(body, "reporterAccountId") {
		t.Fatalf("docs/swagger.json must never document a reporter_account_id/reporterAccountId field")
	}
}

// TestSwaggerCategoryEnumsSplitPostableFromStored: the POST request lists the
// nine categories a new report may use (service.PostableCategories), while the
// response shapes and the GET category filter list all ten known categories
// (service.Categories), because a report stored as rescue_needed before
// 2026-10-16 can still be read and filtered. Order is part of the contract.
func TestSwaggerCategoryEnumsSplitPostableFromStored(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("reading docs/swagger.json: %v", err)
	}
	var spec struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		} `json:"definitions"`
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name string   `json:"name"`
				In   string   `json:"in"`
				Enum []string `json:"enum"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parsing docs/swagger.json: %v", err)
	}

	known := make([]string, 0, len(service.Categories))
	for _, c := range service.Categories {
		known = append(known, string(c))
	}
	postable := make([]string, 0, len(service.PostableCategories))
	for _, c := range service.PostableCategories {
		postable = append(postable, string(c))
	}

	check := func(label string, got, want []string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s enum = %v, want %v", label, got, want)
		}
	}
	check("handlers.SubmitReportRequest category", spec.Definitions["handlers.SubmitReportRequest"].Properties["category"].Enum, postable)
	check("handlers.ReportResponse category", spec.Definitions["handlers.ReportResponse"].Properties["category"].Enum, known)
	check("handlers.FeedReportResponse category", spec.Definitions["handlers.FeedReportResponse"].Properties["category"].Enum, known)

	var found bool
	for _, p := range spec.Paths["/reports"]["get"].Parameters {
		if p.Name == "category" && p.In == "query" {
			found = true
			check("GET /reports category query", p.Enum, known)
		}
	}
	if !found {
		t.Error("GET /reports documents no category query parameter")
	}
}
