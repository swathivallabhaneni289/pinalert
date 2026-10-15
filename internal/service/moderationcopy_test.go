package service_test

import (
	"testing"

	"pinalert/internal/service"
)

func TestOlderNumbersLabel(t *testing.T) {
	cases := []struct {
		name  string
		older []string
		want  string
	}{
		{"default list", []string{"100", "101", "108"}, "Older numbers for police (100), fire (101) and ambulance (108)."},
		{"police and ambulance", []string{"100", "108"}, "Older numbers for police (100) and ambulance (108)."},
		{"police alone", []string{"100"}, "An older number for police (100)."},
		{"unmapped number", []string{"100", "555"}, service.OlderNumbersGenericLabel},
		{"only unmapped", []string{"555"}, service.OlderNumbersGenericLabel},
		{"nil", nil, ""},
		{"empty", []string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.OlderNumbersLabel(tc.older); got != tc.want {
				t.Errorf("OlderNumbersLabel(%v) = %q, want %q", tc.older, got, tc.want)
			}
		})
	}
	if service.OlderNumbersGenericLabel != "Older emergency numbers that many state governments still list." {
		t.Errorf("OlderNumbersGenericLabel = %q", service.OlderNumbersGenericLabel)
	}
}
