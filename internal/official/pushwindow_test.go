package official

import (
	"testing"

	"pinalert/internal/service"
)

// TestOfficialPushFreshWindowMatches pins the send worker's window for "the feed
// is current" (plan 05-16, in internal/service) to the one the feed's freshness
// states use. The two packages cannot share the constant: service never imports
// this package, so a test in this one, which may import service, keeps them
// equal.
func TestOfficialPushFreshWindowMatches(t *testing.T) {
	if service.OfficialPushFreshWindow != FreshWindow {
		t.Fatalf("service.OfficialPushFreshWindow = %v, official.FreshWindow = %v, want them equal",
			service.OfficialPushFreshWindow, FreshWindow)
	}
}
