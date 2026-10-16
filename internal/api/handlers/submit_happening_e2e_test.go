// submit_happening_e2e_test.go proves plan 03.4-02's write over the real router
// and a real Postgres: the What is happening pick is stored for a Critical
// report only, anything else is refused with 400 on the field happening and
// stores nothing, and a queued body written before the field existed still
// posts (D-24, D-24a, D-24b).
package handlers_test

import (
	"context"
	"net/http"
	"testing"
)

func TestSubmitHappening(t *testing.T) {
	srv, mailer, pool := newE2EServer(t)
	client := newVerifiedClient(t, srv, mailer, "submit-happening@example.com")

	body := func(severity string, extra map[string]any) map[string]any {
		b := map[string]any{
			"category":    "fire",
			"severity":    severity,
			"description": "Smoke and flames from the warehouse roof",
			"latitude":    h10Lat,
			"longitude":   h10Lon,
		}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	stored := func(id int64) *string {
		t.Helper()
		var h *string
		if err := pool.QueryRow(context.Background(), `SELECT happening FROM reports WHERE id = $1`, id).Scan(&h); err != nil {
			t.Fatalf("reading happening of report %d: %v", id, err)
		}
		return h
	}
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM reports`).Scan(&n); err != nil {
			t.Fatalf("counting reports: %v", err)
		}
		return n
	}

	t.Run("a critical post stores the pick", func(t *testing.T) {
		id := submitReportGetID(t, client, srv.URL, body("critical", map[string]any{"happening": "people_trapped"}))
		if h := stored(id); h == nil || *h != "people_trapped" {
			t.Errorf("stored happening = %v, want people_trapped", h)
		}
	})

	t.Run("a critical post without the field stores NULL", func(t *testing.T) {
		id := submitReportGetID(t, client, srv.URL, body("critical", nil))
		if h := stored(id); h != nil {
			t.Errorf("stored happening = %q, want NULL", *h)
		}
	})

	t.Run("a medium post with a pick is refused on happening and stores nothing", func(t *testing.T) {
		before := count()
		resp := postReport(t, client, srv.URL, body("medium", map[string]any{"happening": "fire"}))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d, want 400", resp.StatusCode)
		}
		if field, _ := decodeErrorResponse(t, resp); field != "happening" {
			t.Errorf("error field = %q, want happening", field)
		}
		if after := count(); after != before {
			t.Errorf("reports went from %d to %d after a refused post", before, after)
		}
	})

	t.Run("a low post and an unknown or wrong case value are refused on happening", func(t *testing.T) {
		before := count()
		for _, b := range []map[string]any{
			body("low", map[string]any{"happening": "fire"}),
			body("critical", map[string]any{"happening": "earthquake"}),
			body("critical", map[string]any{"happening": "Fire"}),
			body("critical", map[string]any{"happening": ""}),
		} {
			resp := postReport(t, client, srv.URL, b)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d for %v, want 400", resp.StatusCode, b["happening"])
			}
			if field, _ := decodeErrorResponse(t, resp); field != "happening" {
				t.Errorf("error field = %q for %v, want happening", field, b["happening"])
			}
		}
		if after := count(); after != before {
			t.Errorf("reports went from %d to %d after refused posts", before, after)
		}
	})

	t.Run("a queued body from before this phase still posts", func(t *testing.T) {
		// The shape the offline queue wrote before the field existed: no
		// happening key, with a request id and the time the device wrote it.
		queued := body("medium", map[string]any{
			"request_id": "5f0c2a9e-8d1b-4c7e-9a33-2b6f1e4d7c10",
			"written_at": "2026-10-17T09:00:00.000Z",
		})
		resp := postReport(t, client, srv.URL, queued)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d, want 201", resp.StatusCode)
		}
	})
}
