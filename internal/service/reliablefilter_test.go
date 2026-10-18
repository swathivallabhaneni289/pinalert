package service_test

import (
	"context"
	"fmt"
	"testing"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// TestKeptByReliableFilterGrid pins the one rule of the opt in Only Reliable
// reporters tick: a report is kept when its reporter's tag is reliable or the
// report is serious (a critical severity, or a rescue_needed category stored
// before that category was retired). Every other tag and category combination
// is not kept, so the tick can hide it.
func TestKeptByReliableFilterGrid(t *testing.T) {
	severities := []service.Severity{service.SeverityLow, service.SeverityMedium, service.SeverityCritical}
	categories := []service.Category{service.CategoryFlood, service.CategoryTraffic, service.CategoryRescueNeeded}
	for _, tag := range service.ReporterTags {
		for _, sev := range severities {
			for _, cat := range categories {
				want := tag == service.ReporterReliable || sev == service.SeverityCritical || cat == service.CategoryRescueNeeded
				meta := service.ReportMeta{Severity: sev, Category: cat}
				name := fmt.Sprintf("%s %s %s", tag, sev, cat)
				t.Run(name, func(t *testing.T) {
					got := service.KeptByReliableFilter(tag, meta)
					if got != want {
						t.Errorf("KeptByReliableFilter(%s, %s/%s) = %v, want %v", tag, sev, cat, got, want)
					}
					if got != (tag == service.ReporterReliable || meta.Serious()) {
						t.Errorf("KeptByReliableFilter disagrees with ReportMeta.Serious() for %s", name)
					}
				})
			}
		}
	}
}

// TestNearbyCarriesKeptByReliableFilter: every Nearby report carries the server's
// answer, read from the same trust object the report carries, and the field
// filters nothing on the server.
func TestNearbyCarriesKeptByReliableFilter(t *testing.T) {
	hist7, votes7 := p12Corroborated(7, 701, 5)
	fake := &fakeFeedQuerier{
		nearbyRows: []sqlcgen.NearbyReportsRow{
			p12Row(1, service.SeverityLow, service.CategoryFlood, 1),
			p12Row(2, service.SeverityLow, service.CategoryFlood, 2),
			p12Row(3, service.SeverityCritical, service.CategoryFlood, 3),
			p12Row(4, service.SeverityLow, service.CategoryRescueNeeded, 4),
			p12Row(5, service.SeverityLow, service.CategoryFlood, 5),
		},
		reporterRows: []sqlcgen.ReporterAccountsForReportsRow{
			reporterRow(1, ptrID(7)),
			reporterRow(2, ptrID(8)),
			reporterRow(3, ptrID(8)),
			reporterRow(4, ptrID(8)),
			reporterRow(5, nil),
		},
		historyRows: hist7,
		voteRows:    votes7,
	}
	svc := service.NewReportService(fake, service.WithReportClock(p12Clock))
	views, err := svc.Nearby(context.Background(), service.NearbyQuery{Latitude: 12.9716, Longitude: 77.5946, RadiusKm: 5})
	if err != nil {
		t.Fatalf("Nearby: %v", err)
	}
	want := map[int64]bool{
		1: true,  // a reliable reporter's low report
		2: false, // a new reporter's low report
		3: true,  // a new reporter's critical report
		4: true,  // an old rescue_needed report
		5: false, // a report with no account
	}
	if len(views) != len(want) {
		t.Fatalf("Nearby listed %v, want all %d reports (the field filters nothing)", viewIDs(views), len(want))
	}
	if got := p12ReporterOf(t, views, 1); got.Tag != service.ReporterReliable {
		t.Fatalf("report 1 reporter tag = %q, want reliable", got.Tag)
	}
	for id, w := range want {
		v, ok := findView(views, id)
		if !ok {
			t.Fatalf("report %d missing", id)
		}
		if v.KeptByReliableFilter != w {
			t.Errorf("report %d KeptByReliableFilter = %v, want %v (tag %q)", id, v.KeptByReliableFilter, w, v.Trust.Reporter.Tag)
		}
	}
}
