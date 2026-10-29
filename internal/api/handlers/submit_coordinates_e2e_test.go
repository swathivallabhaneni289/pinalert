// submit_coordinates_e2e_test.go pins backlog 999.11: a report needs both
// coordinates. An absent key or a JSON null is refused with 400 naming the
// field, and an explicit 0 is a real place that is still accepted.
package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	sqlcgen "pinalert/internal/store/sqlc"
)

func TestSubmitReportRefusesMissingCoordinates(t *testing.T) {
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.RateLimits.ReportOther = api.BucketLimit{Every: time.Hour, Burst: 2}
		d.RateLimits.ReportSerious = api.BucketLimit{Every: time.Hour, Burst: 2}
	})

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	verifySession(t, client, srv, mailer, "coords-5tq@example.com")

	body := func(extra map[string]any) map[string]any {
		b := map[string]any{
			"category":    "flood",
			"severity":    "low",
			"description": "Water over the road near the bridge",
		}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}

	rows := []struct {
		name    string
		extra   map[string]any
		field   string
		message string
	}{
		{"latitude absent", map[string]any{"longitude": 77.5946}, "latitude", "latitude is required."},
		{"longitude absent", map[string]any{"latitude": 12.9716}, "longitude", "longitude is required."},
		{"latitude null", map[string]any{"latitude": nil, "longitude": 77.5946}, "latitude", "latitude is required."},
		{"longitude null", map[string]any{"latitude": 12.9716, "longitude": nil}, "longitude", "longitude is required."},
		{"both absent", map[string]any{}, "latitude", "latitude is required."},
		{"both null", map[string]any{"latitude": nil, "longitude": nil}, "latitude", "latitude is required."},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			resp := postReport(t, client, srv.URL, body(row.extra))
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s: status = %d, want 400; body=%s", row.name, resp.StatusCode, raw)
			}
			var out handlers.ErrorResponse
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("%s: decoding %s: %v", row.name, raw, err)
			}
			if out.Error.Field != row.field || out.Error.Message != row.message {
				t.Fatalf("%s: field=%q message=%q, want field=%q message=%q; body=%s",
					row.name, out.Error.Field, out.Error.Message, row.field, row.message, raw)
			}
		})
	}

	// The expected_account_id check still runs first, as the route promises.
	conflict := body(map[string]any{"longitude": 77.5946, "expected_account_id": "999999999"})
	resp := postReport(t, client, srv.URL, conflict)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var conflictOut handlers.ErrorResponse
	if resp.StatusCode != http.StatusConflict || json.Unmarshal(raw, &conflictOut) != nil || conflictOut.Error.Field != "expected_account_id" {
		t.Fatalf("latitude absent with a foreign expected_account_id: status=%d body=%s, want 409 on expected_account_id", resp.StatusCode, raw)
	}

	count := func() int {
		var n int
		if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM reports").Scan(&n); err != nil {
			t.Fatalf("counting reports: %v", err)
		}
		return n
	}
	if n := count(); n != 0 {
		t.Fatalf("reports stored after the refusals = %d, want 0", n)
	}

	// The refusals used no allowance: two good posts still fit the Burst of 2.
	resp = postReport(t, client, srv.URL, body(map[string]any{"latitude": 0, "longitude": 0}))
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("explicit 0, 0 post: status = %d, want 201; body=%s", resp.StatusCode, raw)
	}
	var created handlers.SubmitReportResponse
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("explicit 0, 0 post: decoding %s: %v", raw, err)
	}
	if created.Report.Latitude != 0 || created.Report.Longitude != 0 {
		t.Fatalf("explicit 0, 0 post: report at %v, %v, want 0, 0", created.Report.Latitude, created.Report.Longitude)
	}

	second := body(map[string]any{"latitude": 12.9716, "longitude": 77.5946})
	second["description"] = "Road shut by a fallen tree at the junction"
	resp = postReport(t, client, srv.URL, second)
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("normal post after the refusals: status = %d, want 201; body=%s", resp.StatusCode, raw)
	}
	if n := count(); n != 2 {
		t.Fatalf("reports stored at the end = %d, want 2", n)
	}
}
