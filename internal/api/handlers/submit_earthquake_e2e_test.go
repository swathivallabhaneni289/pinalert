// submit_earthquake_e2e_test.go proves plan 03.4-29's ninth tile end to end over
// the real router and a real Postgres: the Post a report window now offers
// Earthquake, and a report sent as earthquake is accepted at every severity and
// stored under that category.
package handlers_test

import (
	"context"
	"testing"
)

func TestSubmitEarthquake(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "submit-earthquake@example.com")

	for _, severity := range []string{"low", "medium", "critical"} {
		t.Run(severity, func(t *testing.T) {
			id := submitReportGetID(t, client, srv.URL, map[string]any{
				"category":    "earthquake",
				"severity":    severity,
				"description": "The ground shook for about ten seconds, a wall has cracked",
				"latitude":    h10Lat,
				"longitude":   h10Lon,
			})
			var category, stored string
			if err := pool.QueryRow(context.Background(), `SELECT category, severity FROM reports WHERE id = $1`, id).Scan(&category, &stored); err != nil {
				t.Fatalf("reading report %d: %v", id, err)
			}
			if category != "earthquake" {
				t.Errorf("stored category = %q, want earthquake", category)
			}
			if stored != severity {
				t.Errorf("stored severity = %q, want %q", stored, severity)
			}
		})
	}
}
