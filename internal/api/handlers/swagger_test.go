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

	body := string(raw)

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

	// The published-schema counterpart to the runtime information-disclosure
	// control (T-01-02): a session identifier must never be advertised as
	// part of the contract, because a documented field is an invitation for
	// a future change to start populating it.
	if strings.Contains(body, "session_id") || strings.Contains(body, "sessionId") {
		t.Fatalf("docs/swagger.json must never document a session_id/sessionId field")
	}
}
