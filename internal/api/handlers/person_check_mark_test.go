package handlers

// person_check_mark_test.go pins the profile's side of quick 261118-saj: a live
// row carries the Being checked mark when the view says so, and a finished row
// never does.

import (
	"testing"
	"time"

	"pinalert/internal/service"
)

func TestPersonReportRowsBeingChecked(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	view := func(checked bool) service.ReportView {
		return service.ReportView{
			Report: service.Report{
				ID: 7, Category: service.CategoryFlood, Severity: service.SeverityCritical,
				Description: "Bridge washed out", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
			},
			BeingChecked: checked,
		}
	}
	rows := personReportRows([]service.ProfileReportRow{
		{View: view(true), Live: true, Status: service.ProfileStatusOngoing},
		{View: view(false), Live: true, Status: service.ProfileStatusOngoing},
		{View: view(true), Live: false, Status: service.ProfileStatusExpired},
	}, now)
	if !rows[0].BeingChecked {
		t.Error("a marked live row does not carry the mark")
	}
	if rows[1].BeingChecked {
		t.Error("an unmarked live row carries the mark")
	}
	if rows[2].BeingChecked {
		t.Error("a finished row carries the mark")
	}
}
