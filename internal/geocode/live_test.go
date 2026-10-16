package geocode

// The live check for quick task 261016-pho. It is the only test in this
// package that calls the real Photon and Nominatim, and it is off unless
// GEOCODE_LIVE=1, the same way MODERATION_LIVE gates the moderation recorder.
// Run it by hand, rarely:
//
//	GEOCODE_LIVE=1 go test ./internal/geocode/ -run TestGeocodeLive -count=1 -v
//
// It sends at most one request a second to each service (both limiters are
// set to one a second here, and it pauses between searches) and prints a
// table of what each search found and which service answered.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestGeocodeLive(t *testing.T) {
	if os.Getenv("GEOCODE_LIVE") != "1" {
		t.Skip("live check calls the real Photon and Nominatim; run with GEOCODE_LIVE=1")
	}

	hyderabad := &Point{Lat: 17.385, Lon: 78.4867}
	delhi := &Point{Lat: 28.6139, Lon: 77.2090}
	cases := []struct {
		typed   string
		near    *Point
		pinName string
	}{
		{"Banjra Hills", hyderabad, "Hyderabad"},
		{"Hyderbad", hyderabad, "Hyderabad"},
		{"Banjara Hi", hyderabad, "Hyderabad"},
		{"Gachib", hyderabad, "Hyderabad"},
		{"Road No 12 Banjara Hills Hyderabad", hyderabad, "Hyderabad"},
		{"Flat 4B, Sai Residency, Road No 12, Banjara Hills, Hyderabad", hyderabad, "Hyderabad"},
		{"Hyderabad", delhi, "Delhi"},
		{"Ring Road", hyderabad, "Hyderabad"},
		{"హైదరాబాద్", hyderabad, "Hyderabad"},
		{"Hyderbad", nil, "none"},
	}

	c := newClientWithProviders(photonSearchURL, nominatimSearchURL, os.Getenv("NOMINATIM_CONTACT_EMAIL"),
		time.Second, 1, time.Second, 1)

	rows := []string{
		"| Typed | Pin | Answered by | Took | Matched text | First results |",
		"| --- | --- | --- | --- | --- | --- |",
	}
	for i, tc := range cases {
		if i > 0 {
			// Keep well inside one request a second to each service.
			time.Sleep(1200 * time.Millisecond)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		start := time.Now()
		answer, err := c.Search(ctx, tc.typed, tc.near)
		took := time.Since(start).Round(10 * time.Millisecond)
		cancel()
		if err != nil {
			t.Errorf("%q: %v", tc.typed, err)
			rows = append(rows, "| "+tc.typed+" | "+tc.pinName+" | error | "+took.String()+" | | "+err.Error()+" |")
			continue
		}
		var first []string
		for j, r := range answer.Results {
			if j == 3 {
				break
			}
			label := r.DisplayName
			if r.Name != "" && !strings.HasPrefix(label, r.Name) {
				label = r.Name + " (" + label + ")"
			}
			first = append(first, label)
		}
		results := strings.Join(first, "; ")
		if results == "" {
			results = "nothing"
		}
		provider := answer.provider
		if provider == "" {
			provider = "cache"
		}
		rows = append(rows, "| "+tc.typed+" | "+tc.pinName+" | "+provider+" | "+took.String()+" | "+answer.MatchedQuery+" | "+results+" |")
	}
	t.Log("\n" + strings.Join(rows, "\n"))
}
