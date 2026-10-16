// category_retire_e2e_test.go covers the 2026-10-16 owner decision that Rescue
// needed is no longer a category for a new report and Traffic took its place.
// Reports already stored as rescue_needed keep every rule they had, so the
// tests of those rules insert such rows straight into the database with
// storeReportAs instead of posting them.
package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/service"
	"pinalert/internal/testutil"
)

// storeReportAs writes the report a test used to POST straight into the
// database, as a row that account's own session posted. The body map is the
// same one postReport takes: category, severity, description, latitude and
// longitude. It returns the new report id.
func storeReportAs(t *testing.T, pool *pgxpool.Pool, client *http.Client, baseURL string, body map[string]any) int64 {
	t.Helper()

	str := func(key string) string {
		v, ok := body[key].(string)
		if !ok {
			t.Fatalf("storeReportAs: body[%q] = %#v, want a string", key, body[key])
		}
		return v
	}
	num := func(key string) float64 {
		v, ok := body[key].(float64)
		if !ok {
			t.Fatalf("storeReportAs: body[%q] = %#v, want a float64", key, body[key])
		}
		return v
	}

	severity := str("severity")
	createdAt := time.Now().UTC()
	return testutil.InsertStoredReport(t, pool, testutil.StoredReport{
		SessionID:       sessionIDFromJar(t, client, baseURL),
		Category:        str("category"),
		Severity:        severity,
		Description:     strings.TrimSpace(str("description")),
		Latitude:        num("latitude"),
		Longitude:       num("longitude"),
		CreatedAt:       createdAt,
		ExpiresAt:       createdAt.Add(service.ExpiryDuration(service.Severity(severity))),
		ModerationState: "clear",
	})
}
