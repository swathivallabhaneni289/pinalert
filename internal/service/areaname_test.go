package service_test

// areaname_test.go proves the shared area name lookup with fakes: the attempt
// is recorded before the outside call, a failed call stores nothing, the name
// is stored trimmed, and no namer means no call of any kind.

import (
	"context"
	"errors"
	"testing"
	"time"

	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

type areaStoreFake struct {
	events   *[]string
	markErr  error
	storedAs []string
}

func (f *areaStoreFake) MarkAreaLookupTried(_ context.Context, arg sqlcgen.MarkAreaLookupTriedParams) error {
	*f.events = append(*f.events, "mark")
	return f.markErr
}

func (f *areaStoreFake) SetReportAreaName(_ context.Context, arg sqlcgen.SetReportAreaNameParams) error {
	*f.events = append(*f.events, "set")
	f.storedAs = append(f.storedAs, arg.AreaName)
	return nil
}

type areaNamerFake struct {
	events *[]string
	name   string
	err    error
	calls  int
}

func (f *areaNamerFake) ReverseArea(_ context.Context, _, _ float64) (string, error) {
	*f.events = append(*f.events, "namer")
	f.calls++
	return f.name, f.err
}

func TestLookUpReportAreaName(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

	t.Run("the attempt is recorded before the namer is called", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events}
		namer := &areaNamerFake{events: &events, name: "Benz Circle"}
		got := service.LookUpReportAreaName(context.Background(), store, namer, 7, 16.5, 80.6, now)
		if got != "Benz Circle" {
			t.Errorf("name = %q, want Benz Circle", got)
		}
		want := []string{"mark", "namer", "set"}
		if len(events) != 3 || events[0] != want[0] || events[1] != want[1] || events[2] != want[2] {
			t.Errorf("events = %v, want %v", events, want)
		}
	})

	t.Run("a failing namer leaves no stored name", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events}
		namer := &areaNamerFake{events: &events, err: errors.New("lookup down")}
		got := service.LookUpReportAreaName(context.Background(), store, namer, 7, 16.5, 80.6, now)
		if got != "" {
			t.Errorf("name = %q, want empty", got)
		}
		if len(store.storedAs) != 0 {
			t.Errorf("SetReportAreaName called %d times, want 0", len(store.storedAs))
		}
		if len(events) != 2 || events[0] != "mark" {
			t.Errorf("events = %v, want the attempt then the call", events)
		}
	})

	t.Run("an empty answer stores nothing", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events}
		namer := &areaNamerFake{events: &events, name: "   "}
		if got := service.LookUpReportAreaName(context.Background(), store, namer, 7, 16.5, 80.6, now); got != "" {
			t.Errorf("name = %q, want empty", got)
		}
		if len(store.storedAs) != 0 {
			t.Errorf("SetReportAreaName called %d times, want 0", len(store.storedAs))
		}
	})

	t.Run("the name is stored trimmed", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events}
		namer := &areaNamerFake{events: &events, name: "  Benz Circle "}
		got := service.LookUpReportAreaName(context.Background(), store, namer, 7, 16.5, 80.6, now)
		if got != "Benz Circle" {
			t.Errorf("returned %q, want Benz Circle", got)
		}
		if len(store.storedAs) != 1 || store.storedAs[0] != "Benz Circle" {
			t.Errorf("stored %v, want [Benz Circle]", store.storedAs)
		}
	})

	t.Run("a failed attempt write makes no outside call", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events, markErr: errors.New("db down")}
		namer := &areaNamerFake{events: &events, name: "Benz Circle"}
		if got := service.LookUpReportAreaName(context.Background(), store, namer, 7, 16.5, 80.6, now); got != "" {
			t.Errorf("name = %q, want empty", got)
		}
		if namer.calls != 0 {
			t.Errorf("namer called %d times, want 0", namer.calls)
		}
	})

	t.Run("a nil namer makes no call at all", func(t *testing.T) {
		var events []string
		store := &areaStoreFake{events: &events}
		if got := service.LookUpReportAreaName(context.Background(), store, nil, 7, 16.5, 80.6, now); got != "" {
			t.Errorf("name = %q, want empty", got)
		}
		if len(events) != 0 {
			t.Errorf("events = %v, want none", events)
		}
	})
}
