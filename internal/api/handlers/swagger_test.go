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

	// 02-04's contribution to this same OPS-01 drift guard: one response
	// shape changed (GET /reports now returns FeedReportResponse, not
	// ReportResponse), one spec regenerated. The GET /reports operation
	// must declare the show_disputed parameter.
	if getOp, ok := reportsOps["get"]; ok {
		if !strings.Contains(string(getOp), "show_disputed") {
			t.Errorf("docs/swagger.json GET /reports operation is missing the show_disputed parameter")
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
		"power_outage", "shelter_open", "rescue_needed", "other",
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
