package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/official"
)

// stubOfficial is a local OfficialReader double.
type stubOfficial struct {
	view   official.View
	err    error
	gotAcc int64
	gotLat *float64
	gotLon *float64
	alerts int
	kicks  int
}

func (s *stubOfficial) Alerts(_ context.Context, accountID int64, lat, lon *float64) (official.View, error) {
	s.alerts++
	s.gotAcc, s.gotLat, s.gotLon = accountID, lat, lon
	return s.view, s.err
}

func (s *stubOfficial) Kick() { s.kicks++ }

func officialRequest(t *testing.T, h http.Handler, target string, withAccount bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if withAccount {
		req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 17}))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOfficialAlertsValidatesPosition(t *testing.T) {
	cases := []struct {
		name, query, field string
	}{
		{"lat without lon", "?lat=16.5", "lon"},
		{"lon without lat", "?lon=80.6", "lat"},
		{"lat out of range", "?lat=91&lon=80.6", "lat"},
		{"lon out of range", "?lat=16.5&lon=181", "lon"},
		{"lat not a number", "?lat=abc&lon=80.6", "lat"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &stubOfficial{}
			rec := officialRequest(t, OfficialAlerts(stub), "/api/official/alerts"+c.query, true)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if body := decodeErrorBody(t, rec); body.Error.Field != c.field {
				t.Errorf("field = %q, want %q", body.Error.Field, c.field)
			}
			if stub.alerts != 0 || stub.kicks != 0 {
				t.Errorf("a refused request must not read or kick: %d %d", stub.alerts, stub.kicks)
			}
		})
	}
}

func TestOfficialAlertsNilServiceDegrades(t *testing.T) {
	rec := officialRequest(t, OfficialAlerts(nil), "/api/official/alerts", true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if body := decodeErrorBody(t, rec); body.Error.Message != "Official alerts are not available right now." {
		t.Errorf("message = %q", body.Error.Message)
	}
}

func TestOfficialAlertsErrorIsNotAnEmptyList(t *testing.T) {
	stub := &stubOfficial{err: errors.New("boom 16.5062 80.6480")}
	rec := officialRequest(t, OfficialAlerts(stub), "/api/official/alerts?lat=16.5062&lon=80.6480", true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "groups") || strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body = %s", rec.Body.String())
	}
	if stub.kicks != 0 {
		t.Errorf("a failed read must not kick")
	}
}

func TestOfficialAlertsNoAccountIsAnInternalError(t *testing.T) {
	rec := officialRequest(t, OfficialAlerts(&stubOfficial{}), "/api/official/alerts", false)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestOfficialAlertsAnswersAndKicks(t *testing.T) {
	last := time.Date(2026, 10, 29, 6, 25, 0, 0, time.UTC)
	age := int64(240)
	stub := &stubOfficial{view: official.View{
		State: official.StateFresh, AgeSeconds: &age, LastSuccessAt: &last, AreaKnown: true, Count: 1,
		DistrictsLine: "Ntr district",
		Groups: []official.Group{{Kind: "reader", Label: "Where you are: Ntr district", Rows: []official.Row{{
			ID: 41, Source: "sachet", Level: "severe", LevelWord: "Severe", Badge: "Official", Headline: "Heavy rain",
			Issuer: "IMD Chennai", IssuedAt: last, ExpiresAt: last.Add(2 * time.Hour), GroupedCount: 1,
			Details: official.Details{SourceLine: "NDMA SACHET"},
		}}}},
		Banner: &official.Banner{Level: "severe", LevelWord: "Severe", EventPhrase: "Heavy rain warning", Area: "Ntr district", More: 0, AlertID: 41},
	}}
	rec := officialRequest(t, OfficialAlerts(stub), "/api/official/alerts?lat=16.5062&lon=80.6480", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if stub.kicks != 1 || stub.gotAcc != 17 {
		t.Errorf("kicks %d account %d", stub.kicks, stub.gotAcc)
	}
	if stub.gotLat == nil || stub.gotLon == nil || *stub.gotLat != 16.5062 || *stub.gotLon != 80.6480 {
		t.Errorf("position not passed: %v %v", stub.gotLat, stub.gotLon)
	}

	var got OfficialAlertsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got.State != "fresh" || got.AgeSeconds == nil || *got.AgeSeconds != 240 || !got.AreaKnown || got.Count != 1 {
		t.Errorf("response = %+v", got)
	}
	if len(got.Groups) != 1 || got.Groups[0].Rows[0].ID != 41 || got.Groups[0].Rows[0].IssuedAt != "2026-10-29T06:25:00Z" {
		t.Errorf("groups = %+v", got.Groups)
	}
	if got.Banner == nil || got.Banner.EventPhrase != "Heavy rain warning" {
		t.Errorf("banner = %+v", got.Banner)
	}
}

func TestOfficialAlertsNoPositionPassesNil(t *testing.T) {
	stub := &stubOfficial{view: official.View{State: official.StateFresh, Groups: []official.Group{}}}
	rec := officialRequest(t, OfficialAlerts(stub), "/api/official/alerts", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if stub.gotLat != nil || stub.gotLon != nil {
		t.Errorf("a request with no position must pass nil")
	}
	// Arrays are [] and a missing time or banner is null.
	body := rec.Body.String()
	for _, want := range []string{`"groups":[]`, `"banner":null`, `"last_success_at":null`, `"age_seconds":null`} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %s: %s", want, body)
		}
	}
}
