// category_retire_e2e_test.go covers the 2026-10-16 owner decision that Rescue
// needed is no longer a category for a new report and Traffic took its place.
// Reports already stored as rescue_needed keep every rule they had, so the
// tests of those rules insert such rows straight into the database with
// storeReportAs instead of posting them.
package handlers_test

import (
	"encoding/json"
	"io"
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

// crBody is the report body these tests post, with the category and severity
// they vary. The coordinates are the h10 centre, so a read from there sees it.
func crBody(category, severity string) map[string]any {
	return map[string]any{
		"category":    category,
		"severity":    severity,
		"description": "Slow traffic on the ring road near the bridge",
		"latitude":    h10Lat,
		"longitude":   h10Lon,
	}
}

// TestSubmitRefusesRetiredRescueCategory: a new report cannot be rescue_needed
// at any severity. The answer is 400 on the field category with the plain
// words message, and nothing is stored (T-w8t-01).
func TestSubmitRefusesRetiredRescueCategory(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "cr-refuse@example.com")

	for _, severity := range []string{"low", "medium", "critical"} {
		resp := postReport(t, client, srv.URL, crBody("rescue_needed", severity))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("severity %s: status = %d, want 400", severity, resp.StatusCode)
		}
		field, message := decodeErrorResponse(t, resp)
		if field != "category" || message != service.RescueNeededRetiredMessage {
			t.Errorf("severity %s: field %q message %q, want category and %q", severity, field, message, service.RescueNeededRetiredMessage)
		}
	}
	if n := p11ReportCount(t, pool); n != 0 {
		t.Errorf("refused rescue posts stored %d reports, want 0", n)
	}
}

// TestTrafficCategoryPostsAndFilters: Traffic posts, is listed by the category
// filter, and the filter for the retired rescue_needed slug is still accepted
// and lists a row stored before the change.
func TestTrafficCategoryPostsAndFilters(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "cr-traffic@example.com")

	resp := postReport(t, client, srv.URL, crBody("traffic", "low"))
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("traffic post: status = %d, want 201: %s", resp.StatusCode, b)
	}
	var created struct {
		Report struct {
			ID       int64  `json:"id"`
			Category string `json:"category"`
		} `json:"report"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decoding the traffic post: %v", err)
	}
	resp.Body.Close()
	if created.Report.Category != "traffic" {
		t.Fatalf("created category = %q, want traffic", created.Report.Category)
	}
	trafficID := created.Report.ID

	floodID := submitReportGetID(t, client, srv.URL, crBody("flood", "low"))
	rescueID := storeReportAs(t, pool, client, srv.URL, crBody("rescue_needed", "critical"))

	all, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, ""))
	for name, id := range map[string]int64{"traffic": trafficID, "flood": floodID, "rescue": rescueID} {
		if h10Find(all.Reports, id) == nil {
			t.Errorf("the default view lacks the %s report %d", name, id)
		}
	}

	byTraffic, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "category=traffic"))
	if got := h10Find(byTraffic.Reports, trafficID); got == nil || got.Category != "traffic" {
		t.Errorf("category=traffic lacks the traffic report: %+v", byTraffic.Reports)
	}
	for name, id := range map[string]int64{"flood": floodID, "rescue": rescueID} {
		if h10Find(byTraffic.Reports, id) != nil {
			t.Errorf("category=traffic lists the %s report %d", name, id)
		}
	}

	byRescue, _ := h10Get(t, client, h10URL(srv.URL, h10Lat, h10Lon, "category=rescue_needed"))
	if got := h10Find(byRescue.Reports, rescueID); got == nil || got.Category != "rescue_needed" {
		t.Errorf("category=rescue_needed lacks the stored rescue report: %+v", byRescue.Reports)
	}
	if h10Find(byRescue.Reports, trafficID) != nil {
		t.Errorf("category=rescue_needed lists the traffic report %d", trafficID)
	}
}
