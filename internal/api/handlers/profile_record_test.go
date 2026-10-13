package handlers

import (
	"testing"

	"pinalert/internal/service"
)

// TestNewProfileRecord pins the reporter record card of the phone Activity
// page (quick task 261013-lrw): it is built from the account's own reporter
// signal exactly as the server wrote it. The label is copied, the counts are
// the real judged and corroborated numbers, and nothing is invented for an
// account with no signal.
func TestNewProfileRecord(t *testing.T) {
	cases := []struct {
		name       string
		signal     service.ReporterSignal
		wantNil    bool
		wantLabel  string
		wantCounts string
	}{
		{
			name:       "reliable reporter with five of five",
			signal:     service.ReporterSignal{Tag: service.ReporterReliable, Label: "Reliable reporter", JudgedReports: 5, CorroboratedReports: 5},
			wantLabel:  "Reliable reporter",
			wantCounts: "5 of 5 confirmed",
		},
		{
			name:       "new reporter with real zeros",
			signal:     service.ReporterSignal{Tag: service.ReporterNew, Label: "New reporter", JudgedReports: 0, CorroboratedReports: 0},
			wantLabel:  "New reporter",
			wantCounts: "0 of 0 confirmed",
		},
		{
			name:       "mixed reporter keeps the server words and counts",
			signal:     service.ReporterSignal{Tag: service.ReporterMixed, Label: "Mixed record", JudgedReports: 8, CorroboratedReports: 3},
			wantLabel:  "Mixed record",
			wantCounts: "3 of 8 confirmed",
		},
		{
			name:    "no label means no record",
			signal:  service.ReporterSignal{Tag: service.ReporterNone},
			wantNil: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := newProfileRecord(c.signal)
			if c.wantNil {
				if got != nil {
					t.Fatalf("newProfileRecord = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("newProfileRecord = nil, want label %q", c.wantLabel)
			}
			if got.Label != c.wantLabel {
				t.Errorf("Label = %q, want %q", got.Label, c.wantLabel)
			}
			if got.Counts != c.wantCounts {
				t.Errorf("Counts = %q, want %q", got.Counts, c.wantCounts)
			}
		})
	}
}
