package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

// nearbyStatus runs the NearbyReports handler for one query string and
// returns the status, error.field and error.message. The service has no
// querier: every case here is refused with a 400 before the service is
// reached, so a panic means a bad value got past validation. The deferred
// recover turns that panic into a clean test failure.
func nearbyStatus(t *testing.T, query string) (status int, field, message string) {
	t.Helper()
	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("query %q reached the service: %v", query, rec)
		}
	}()

	handler := NearbyReports(service.NewReportService(nil))
	req := httptest.NewRequest(http.MethodGet, "/api/reports?"+query, nil)
	req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 1, Email: "reader@example.com"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	status = rec.Code
	if status == http.StatusBadRequest {
		body := decodeErrorBody(t, rec)
		field, message = body.Error.Field, body.Error.Message
	}
	return status, field, message
}

// TestNearbyRejectsNaNQueryValues pins UF-03-03: NaN is a number to
// strconv.ParseFloat and passes every < and > range check, so it used to
// reach the query and come back as an empty feed. It is a 400 field error.
func TestNearbyRejectsNaNQueryValues(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		field   string
		message string
	}{
		{"lat NaN", "lat=NaN&lon=77.59&radius_km=5", "lat", "lat must be a number."},
		{"lat nan", "lat=nan&lon=77.59&radius_km=5", "lat", "lat must be a number."},
		{"lat plus NaN", "lat=%2BNaN&lon=77.59&radius_km=5", "lat", "lat must be a number."},
		{"lat minus nan", "lat=-nan&lon=77.59&radius_km=5", "lat", "lat must be a number."},
		{"lon NaN", "lat=12.97&lon=NaN&radius_km=5", "lon", "lon must be a number."},
		{"lon nan", "lat=12.97&lon=nan&radius_km=5", "lon", "lon must be a number."},
		{"lon plus NaN", "lat=12.97&lon=%2BNaN&radius_km=5", "lon", "lon must be a number."},
		{"lon minus nan", "lat=12.97&lon=-nan&radius_km=5", "lon", "lon must be a number."},
		{"radius NaN", "lat=12.97&lon=77.59&radius_km=NaN", "radius_km", "radius_km must be a number."},
		{"radius nan", "lat=12.97&lon=77.59&radius_km=nan", "radius_km", "radius_km must be a number."},
		{"radius plus NaN", "lat=12.97&lon=77.59&radius_km=%2BNaN", "radius_km", "radius_km must be a number."},
		{"radius minus nan", "lat=12.97&lon=77.59&radius_km=-nan", "radius_km", "radius_km must be a number."},
		// Controls: these answered correctly before the change too.
		{"lat text", "lat=abc&lon=77.59&radius_km=5", "lat", "lat must be a number."},
		{"lat Inf", "lat=Inf&lon=77.59&radius_km=5", "lat", "lat is out of range."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, field, message := nearbyStatus(t, tc.query)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", status)
			}
			if field != tc.field {
				t.Errorf("field = %q, want %q", field, tc.field)
			}
			if message != tc.message {
				t.Errorf("message = %q, want %q", message, tc.message)
			}
		})
	}
}
