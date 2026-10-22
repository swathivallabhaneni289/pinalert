package geocode

// Tests for the near only rule of plan 04-32 (04-UAT test 11). The rows below
// are the ones Photon gave live on 2026-10-23 near Benz Circle, Vijayawada,
// with their real positions; every upstream here is an httptest stand in.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// benzCircle is where the owner searched from.
var benzCircle = &Point{Lat: 16.4999, Lon: 80.6567}

// poiRow is a point of interest as Photon sends one.
func poiRow(name string, lat, lon float64, city string) map[string]any {
	return photonFeatureJSON(lon, lat, map[string]any{
		"name": name, "city": city, "countrycode": "IN",
		"osm_key": "shop", "osm_value": "mall", "type": "house",
	})
}

// farMalls are the rows "PVP Square Mall" was answered with: malls of similar
// names, none of them within 1,000 km of Benz Circle.
func farMalls() []map[string]any {
	return []map[string]any{
		poiRow("Vasant Square Mall", 28.524325, 77.156601, "Delhi"),
		poiRow("DB Mall Square", 23.2337806, 77.4302206, "Bhopal"),
		poiRow("Goodwill Square Mall Road", 18.5980943, 73.9042359, "Pune"),
		poiRow("Park Square Mall", 12.9869846, 77.7364154, "Bengaluru"),
		poiRow("Market Square Mall", 12.9218615, 77.6203901, "Bengaluru"),
		poiRow("Square Feet Property Consultant", 19.2208152, 72.8639957, "Mumbai"),
	}
}

// villageRow is a village as Photon sends one.
func villageRow(name string, lat, lon float64) map[string]any {
	return photonFeatureJSON(lon, lat, map[string]any{
		"name": name, "state": "Andhra Pradesh", "countrycode": "IN",
		"osm_key": "place", "osm_value": "village", "type": "city",
	})
}

func TestNearOnlyRowsPastTheFirstFiveStayHidden(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			// Live on 2026-10-23: Photon's first five for "PVP Square Mall"
			// near Benz Circle were the far malls, and its rows six to ten were
			// near villages whose names only start with "Mall". The ranking's
			// cut always hid those guesses, and dropping the far rows must not
			// bring them up: nothing near matches.
			rows := append(farMalls()[:5],
				villageRow("Mallapalle", 16.3999213, 80.7839862),
				villageRow("Mallipoinpalle", 16.7018271, 80.7702398),
				villageRow("Mallikarjunpet", 16.3129605, 80.4469641),
				villageRow("Mallavilli", 16.6716871, 80.8881283),
				villageRow("Mallavaram", 16.3431628, 80.3756936),
			)
			ps := newPhotonStub(t, photonAnswers(t, rows...))
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "PVP Square Mall", benzCircle)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if len(answer.Results) != 0 {
				t.Fatalf("results = %q, want none: nothing near matches", resultNames(answer))
			}
			if len(rs.sent()) != 1 {
				t.Errorf("nominatim asked %d times, want 1 with the whole text", len(rs.sent()))
			}
		})
	}
}

func TestNearOnlyKeepsNearRowsOfTheFirstFiveInOrder(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			malls := farMalls()
			rows := []map[string]any{
				malls[0],
				poiRow("PVP Square Mall Parking", 16.5034, 80.6458, "Vijayawada"),
				malls[1],
				poiRow("Square Mall Bus Stop", 16.4950, 80.6600, "Vijayawada"),
				malls[2],
			}
			ps := newPhotonStub(t, photonAnswers(t, rows...))
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "PVP Square Mall", benzCircle)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			got := resultNames(answer)
			want := []string{"PVP Square Mall Parking", "Square Mall Bus Stop"}
			if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
				t.Fatalf("results = %q, want %q", got, want)
			}
			if len(rs.sent()) != 0 {
				t.Errorf("nominatim asked %d times, want 0: Photon found near rows", len(rs.sent()))
			}
		})
	}
}

func TestNearOnlyFarOnlyAnswerIsNothingFound(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, photonAnswers(t, farMalls()...))
			// Nominatim, asked next, also answers only a far row (Banjara Hills,
			// Hyderabad, 250 km away).
			rs := newRecordingServer(t, func(url.Values) bool { return true })
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "PVP Square Mall", benzCircle)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if len(answer.Results) != 0 || answer.MatchedQuery != "" {
				t.Fatalf("answer = %+v, want nothing found and no matched text", answer)
			}
			if answer.Results == nil {
				t.Errorf("results are nil, want an empty list so the JSON says []")
			}
			// Far rows only count as Photon finding nothing, so the Nominatim
			// fallback is asked once with the whole text, as before.
			sent := rs.sent()
			if len(sent) != 1 || sent[0].Get("q") != "PVP Square Mall" {
				t.Fatalf("nominatim requests = %d (%q), want 1 with the whole text", len(sent), rs.texts())
			}
		})
	}
}

func TestNearOnlyKeepsTheBenzCircleExamplesInOrder(t *testing.T) {
	// "Kanaka Durga Temple", live: the temple and its ghat road in Vijayawada,
	// then temples of the same name 70, 140 and 170 km away.
	rows := []map[string]any{
		poiRow("kanaka durga temple", 16.5154058, 80.6065339, "Vijayawada"),
		poiRow("Kanaka Durga Temple Ghat Road", 16.5136863, 80.6038944, "Vijayawada"),
		poiRow("Kanaka Durga Temple", 16.7841299, 81.2283564, "Gundugolanu"),
		poiRow("Kanaka Durga Temple", 16.4240008, 81.9599846, "Odalarevu"),
		poiRow("KANAKA DURGA TEMPLE", 17.0406708, 82.20997, "Unduru"),
	}
	ps := newPhotonStub(t, photonAnswers(t, rows...))
	c := newDualClient(ps.server.URL, newRecordingServer(t, nil).server.URL)

	answer, err := c.SearchReader(context.Background(), "Kanaka Durga Temple", benzCircle)
	if err != nil {
		t.Fatalf("SearchReader returned error: %v", err)
	}
	got := resultNames(answer)
	if len(got) != 2 || got[0] != "kanaka durga temple" || got[1] != "Kanaka Durga Temple Ghat Road" {
		t.Fatalf("results = %q, want the temple and its ghat road, in that order", got)
	}
}

func TestNoPositionStaysIndiaWide(t *testing.T) {
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, photonAnswers(t, farMalls()...))
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "PVP Square Mall", nil)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if len(answer.Results) != maxResults {
				t.Fatalf("results = %q, want %d rows: with no position nothing is dropped", resultNames(answer), maxResults)
			}
		})
	}
}

func TestFarCityNamedAsTypedIsKept(t *testing.T) {
	t.Run("photon", func(t *testing.T) {
		// Hyderabad the city is 250 km from Benz Circle; a far shop spelled
		// like it is not a city and is dropped.
		shop := poiRow("Hyderabad", 17.39, 78.48, "Hyderabad")
		ps := newPhotonStub(t, photonAnswers(t, shop, indianCity("Hyderabad")))
		c := newDualClient(ps.server.URL, newRecordingServer(t, nil).server.URL)

		answer, err := c.SearchReader(context.Background(), "Hyderbad", benzCircle)
		if err != nil {
			t.Fatalf("SearchReader returned error: %v", err)
		}
		if len(answer.Results) != 1 || answer.Results[0].DisplayName != "Hyderabad, Telangana" {
			t.Fatalf("results = %+v, want only the city of Hyderabad", answer.Results)
		}
	})
	t.Run("nominatim", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"name":"Hyderabad","display_name":"Hyderabad, Telangana, India","lat":"17.3850","lon":"78.4867","category":"place","addresstype":"city"},` +
				`{"name":"Hyderabad Biryani House","display_name":"Hyderabad Biryani House, Pune, Maharashtra, India","lat":"18.52","lon":"73.85","category":"amenity","addresstype":"amenity"}]`))
		}))
		t.Cleanup(server.Close)
		c := newClientWithBaseURL(server.URL, "", 0, 10)

		answer, err := c.Search(context.Background(), "Hyderabad", benzCircle)
		if err != nil {
			t.Fatalf("Search returned error: %v", err)
		}
		if len(answer.Results) != 1 || answer.Results[0].Name != "Hyderabad" {
			t.Fatalf("results = %q, want only the city of Hyderabad", resultNames(answer))
		}
	})
}

func TestAnswerable(t *testing.T) {
	ctx := context.Background()
	// 31 km from Benz Circle, and 250 km.
	near, far := Point{Lat: 16.7, Lon: 80.85}, Point{Lat: 17.385, Lon: 78.4867}
	cases := []struct {
		name      string
		ctx       context.Context
		near      *Point
		at        Point
		cityLevel bool
		rowName   string
		want      bool
	}{
		{"no position keeps a far row", ctx, nil, far, false, "Vasant Square Mall", true},
		{"a near row is kept", ctx, benzCircle, near, false, "PVP Square Mall Parking", true},
		{"a far shop is dropped", ctx, benzCircle, far, false, "PVP Square Mall", false},
		{"a far city spelled otherwise is dropped", ctx, benzCircle, far, true, "Hyderabad", false},
		{"a far city named as typed is kept", ctx, benzCircle, far, true, "PVP Square Mall", true},
		{"the home area search is left as it was", withAreasOnly(ctx), benzCircle, far, false, "Somajiguda", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := answerable(tc.ctx, "PVP Square Mall", tc.near, tc.at.Lat, tc.at.Lon, tc.cityLevel, tc.rowName)
			if got != tc.want {
				t.Fatalf("answerable = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNearRadiusIsFiftyKilometres(t *testing.T) {
	// Babai Hotel in Kaza, 30 km from Benz Circle, was answered live and must
	// stay; the one in Bachupally, Hyderabad, 250 km away, must go.
	if d := distanceKm(benzCircle.Lat, benzCircle.Lon, 16.3883897, 80.5360615); d > nearRadiusKm {
		t.Fatalf("Kaza is %.1f km away, past nearRadiusKm %.0f", d, nearRadiusKm)
	}
	if d := distanceKm(benzCircle.Lat, benzCircle.Lon, 17.5446455, 78.3637161); d <= nearRadiusKm {
		t.Fatalf("Bachupally is %.1f km away, inside nearRadiusKm %.0f", d, nearRadiusKm)
	}
	if nearRadiusKm != 50 {
		t.Fatalf("nearRadiusKm = %v, want 50 (plan 04-32)", nearRadiusKm)
	}
}

func TestTextsForSkipsOneWordShorterTextsNearAPosition(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		ctx  context.Context
		q    string
		near *Point
		want []string
	}{
		{"near: one word left over is not tried", ctx, "PVP Square Mall", benzCircle, []string{"PVP Square Mall", "Square Mall"}},
		{"near: two words only, nothing shorter", ctx, "PVP Square", benzCircle, []string{"PVP Square"}},
		{"near: address parts are still tried", ctx, longAddress, benzCircle, searchTexts(longAddress)},
		{"no position: every shorter text, as before", ctx, "PVP Square Mall", nil, []string{"PVP Square Mall", "Square Mall", "Mall"}},
		{"home area search: as before", withAreasOnly(ctx), "PVP Square Mall", benzCircle, []string{"PVP Square Mall", "Square Mall", "Mall"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := textsFor(tc.ctx, tc.q, tc.near)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("textsFor = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPVPSquareMallNearBenzCircleFindsNothing(t *testing.T) {
	// The live trail of 2026-10-23 once far rows were dropped: Photon's rows
	// for the whole text and for "Square Mall" are far malls, Nominatim has
	// nothing, and only "Mall" finds near rows, villages that start with it.
	far := farMalls()[:5]
	villages := []map[string]any{
		villageRow("Mallapalle", 16.3999213, 80.7839862),
		villageRow("Mallavaram", 16.3431628, 80.3756936),
	}
	for name, search := range lanes {
		t.Run(name, func(t *testing.T) {
			ps := newPhotonStub(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("q") == "Mall" {
					_, _ = w.Write(photonBody(t, villages...))
					return
				}
				_, _ = w.Write(photonBody(t, far...))
			})
			rs := newRecordingServer(t, nil)
			c := newDualClient(ps.server.URL, rs.server.URL)

			answer, err := search(c, context.Background(), "PVP Square Mall", benzCircle)
			if err != nil {
				t.Fatalf("search returned error: %v", err)
			}
			if len(answer.Results) != 0 || answer.MatchedQuery != "" {
				t.Fatalf("answer = %q matched %q, want nothing: nothing near is named so", resultNames(answer), answer.MatchedQuery)
			}
			for _, q := range ps.queries {
				if q.Get("q") == "Mall" {
					t.Fatalf("Photon was asked for the one word %q", "Mall")
				}
			}
		})
	}
}
