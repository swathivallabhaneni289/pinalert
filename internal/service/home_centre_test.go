package service_test

// home_centre_test.go pins plan 03.4-33: the start centre of Nearby and the Low
// data view for an account with a home area is the centre of the stored
// precision 5 cell, never anything finer.

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/mmcloughlin/geohash"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// fakeProfileQuerier answers GetAccountProfile from one fixed row.
type fakeProfileQuerier struct {
	service.ProfileQuerier
	row sqlcgen.GetAccountProfileRow
	err error
}

func (f fakeProfileQuerier) GetAccountProfile(context.Context, int64) (sqlcgen.GetAccountProfileRow, error) {
	return f.row, f.err
}

func TestHomeAreaCentre(t *testing.T) {
	ctx := context.Background()
	cell := geohash.EncodeWithPrecision(16.5115, 80.6160, service.HomeAreaCellPrecision)
	wantLat, wantLon := geohash.DecodeCenter(cell)

	t.Run("a stored cell gives its centre", func(t *testing.T) {
		svc := service.NewAccountService(fakeProfileQuerier{row: sqlcgen.GetAccountProfileRow{HomeAreaCell: strPtr(cell)}})
		lat, lon, ok, err := svc.HomeAreaCentre(ctx, 1)
		if err != nil || !ok {
			t.Fatalf("ok = %v, err = %v, want a centre", ok, err)
		}
		if math.Abs(lat-wantLat) > 1e-12 || math.Abs(lon-wantLon) > 1e-12 {
			t.Fatalf("centre = %v, %v, want %v, %v", lat, lon, wantLat, wantLon)
		}
		// The centre of a cell lies inside the cell, so it names the same cell
		// and tells nothing more than the cell does.
		if again := geohash.EncodeWithPrecision(lat, lon, service.HomeAreaCellPrecision); again != cell {
			t.Fatalf("the centre lies in cell %q, want %q", again, cell)
		}
	})

	t.Run("no stored cell is no home area", func(t *testing.T) {
		svc := service.NewAccountService(fakeProfileQuerier{row: sqlcgen.GetAccountProfileRow{}})
		_, _, ok, err := svc.HomeAreaCentre(ctx, 1)
		if err != nil || ok {
			t.Fatalf("ok = %v, err = %v, want no home area and no error", ok, err)
		}
	})

	t.Run("a cell of the wrong shape is no home area", func(t *testing.T) {
		for _, bad := range []string{"", "abc", "tdr1v9", "TDR1V", "tdr!v", "aaaaa"} {
			svc := service.NewAccountService(fakeProfileQuerier{row: sqlcgen.GetAccountProfileRow{HomeAreaCell: strPtr(bad)}})
			if _, _, ok, err := svc.HomeAreaCentre(ctx, 1); ok || err != nil {
				t.Errorf("cell %q: ok = %v, err = %v, want no home area", bad, ok, err)
			}
		}
	})

	t.Run("a read failure is an error", func(t *testing.T) {
		svc := service.NewAccountService(fakeProfileQuerier{err: errors.New("down")})
		if _, _, ok, err := svc.HomeAreaCentre(ctx, 1); ok || err == nil {
			t.Fatalf("ok = %v, err = %v, want an error", ok, err)
		}
	})
}

// TestHomeAreaCentreRoundTrip runs over a real database and skips without
// DATABASE_URL: the point the person picked is stored as a cell, and the centre
// read back is the cell centre, not the picked point.
func TestHomeAreaCentreRoundTrip(t *testing.T) {
	pool := testutil.NewTestDB(t)
	ctx := context.Background()
	svc := service.NewAccountService(sqlcgen.New(pool))
	id := testutil.SeedReporterAccount(t, pool, "home-centre@example.com")

	if _, _, ok, err := svc.HomeAreaCentre(ctx, id); ok || err != nil {
		t.Fatalf("a new account has no home area: ok = %v, err = %v", ok, err)
	}
	const pickedLat, pickedLon = 12.97841234, 77.64081234
	_, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{HomeArea: service.HomeAreaChange{
		Action: service.HomeAreaSet,
		Input:  service.HomeAreaInput{Name: "Indiranagar, Bengaluru", Lat: pickedLat, Lon: pickedLon},
	}})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	lat, lon, ok, err := svc.HomeAreaCentre(ctx, id)
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v, want a centre", ok, err)
	}
	wantLat, wantLon := geohash.DecodeCenter(geohash.EncodeWithPrecision(pickedLat, pickedLon, service.HomeAreaCellPrecision))
	if lat != wantLat || lon != wantLon {
		t.Fatalf("centre = %v, %v, want the cell centre %v, %v", lat, lon, wantLat, wantLon)
	}
	if lat == pickedLat || lon == pickedLon {
		t.Fatalf("the centre must not be the picked point")
	}

	if _, err := svc.UpdateProfile(ctx, id, service.ProfileUpdate{HomeArea: service.HomeAreaChange{Action: service.HomeAreaClear}}); err != nil {
		t.Fatalf("clearing: %v", err)
	}
	if _, _, ok, err := svc.HomeAreaCentre(ctx, id); ok || err != nil {
		t.Fatalf("after clearing: ok = %v, err = %v, want no home area", ok, err)
	}
}
