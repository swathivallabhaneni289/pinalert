// updates_test.go covers the pure decision core of in app updates (plan
// 03.2-02): the closed kind list, DecideUpdate (when an update is written and
// when the cursor only moves silently), MayNotifyAboutReport (the one notify
// predicate) and HourBucket. Every time is injected, so each rule is a table
// row.

package service

import (
	"testing"
	"time"
)

var updatesNow = time.Date(2026, 10, 18, 12, 30, 0, 0, time.UTC)

func updatesPtr(t time.Time) *time.Time { return &t }

func TestUpdateKindsClosed(t *testing.T) {
	if len(UpdateKinds) != 6 {
		t.Errorf("UpdateKinds has %d entries, want 6", len(UpdateKinds))
	}
	if len(UpdateEventKinds) != 5 {
		t.Errorf("UpdateEventKinds has %d entries, want 5", len(UpdateEventKinds))
	}
	for _, k := range []UpdateKind{UpdateReachedLive, UpdateHidden, UpdateResolved, UpdateFading, UpdateReliableNearby, UpdateMore} {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	for _, k := range []UpdateKind{"", "Reached_live", "comment", "REACHED_LIVE"} {
		if k.Valid() {
			t.Errorf("%q should not be valid", k)
		}
		if k.IsEvent() {
			t.Errorf("%q should not be an event", k)
		}
	}
	if UpdateMore.IsEvent() {
		t.Error("more is a rolled up marker and never an event")
	}
	for _, k := range UpdateEventKinds {
		if !k.IsEvent() {
			t.Errorf("%q should be an event", k)
		}
		if k == UpdateMore {
			t.Error("UpdateEventKinds must not hold more")
		}
	}
	cases := []struct {
		v    Visibility
		want UpdateKind
		ok   bool
	}{
		{VisibilityLive, UpdateReachedLive, true},
		{VisibilityHidden, UpdateHidden, true},
		{VisibilityRetracted, UpdateResolved, true},
		{VisibilityProvisional, "", false},
		{VisibilityWithheld, "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := KindForState(c.v)
		if got != c.want || ok != c.ok {
			t.Errorf("KindForState(%q) = %q, %v; want %q, %v", c.v, got, ok, c.want, c.ok)
		}
	}
}

func TestHourBucket(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{name: "mid hour", in: time.Date(2026, 10, 18, 12, 30, 45, 9, time.UTC), want: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)},
		{name: "on the hour", in: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC), want: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)},
		{name: "one nanosecond before the hour", in: time.Date(2026, 10, 18, 11, 59, 59, 999999999, time.UTC), want: time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)},
		{name: "other zone is read in UTC", in: time.Date(2026, 10, 18, 18, 10, 0, 0, ist), want: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got := HourBucket(c.in)
		if !got.Equal(c.want) || got.Location() != time.UTC {
			t.Errorf("%s: HourBucket(%v) = %v, want %v in UTC", c.name, c.in, got, c.want)
		}
	}
}

func TestDecideUpdate(t *testing.T) {
	now := updatesNow
	adopted := now.Add(-3 * time.Hour)
	longAgo := now.Add(-2 * time.Hour)
	recent := now.Add(-20 * time.Minute)
	farExpiry := now.Add(5 * time.Hour)
	soonExpiry := now.Add(40 * time.Minute)

	cursor := func(last Visibility, lastUpdate *time.Time, fadingFor *time.Time) *UpdateCursor {
		return &UpdateCursor{LastState: last, LastUpdateAt: lastUpdate, AdoptedAt: adopted, FadingFor: fadingFor}
	}

	cases := []struct {
		name string
		in   DecideInput
		want Decision
	}{
		{
			name: "muted live report with a different cursor writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Muted: true, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "muted with no cursor writes nothing",
			in:   DecideInput{Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Muted: true, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "muted and fading writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Muted: true, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "withheld with a cursor writes nothing and keeps the cursor",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityWithheld, View: FeedViewNone, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "withheld with no cursor writes no baseline",
			in:   DecideInput{Visibility: VisibilityWithheld, View: FeedViewNone, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "withheld even with an own vote newer than the adoption",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityWithheld, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(now.Add(-time.Minute)), Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "first sight of a live report adopts it silently",
			in:   DecideInput{Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideBaseline, NewState: VisibilityLive},
		},
		{
			name: "first sight of a provisional report adopts provisional",
			in:   DecideInput{Visibility: VisibilityProvisional, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideBaseline, NewState: VisibilityProvisional},
		},
		{
			name: "first sight of a report about to fade is silent",
			in:   DecideInput{Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideBaseline, NewState: VisibilityLive},
		},
		{
			name: "own vote newer than the adoption adopts the new state silently",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(adopted.Add(time.Minute)), Now: now},
			want: Decision{Action: DecideAdopt, NewState: VisibilityLive},
		},
		{
			name: "own Mark resolved is silent",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityRetracted, View: FeedViewResolved, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(now.Add(-time.Minute)), Now: now},
			want: Decision{Action: DecideAdopt, NewState: VisibilityRetracted},
		},
		{
			name: "own vote adopts even when the state did not change",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(now.Add(-time.Minute)), Now: now},
			want: Decision{Action: DecideAdopt, NewState: VisibilityLive},
		},
		{
			name: "own vote exactly at the adoption time is ignored",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(adopted), Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateReachedLive, NewState: VisibilityLive},
		},
		{
			name: "own vote before the adoption is ignored",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, OwnLatestVoteAt: updatesPtr(adopted.Add(-time.Hour)), Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateReachedLive, NewState: VisibilityLive},
		},
		{
			name: "provisional to live with no earlier update announces reached live",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateReachedLive, NewState: VisibilityLive},
		},
		{
			name: "live to hidden announces hidden",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityHidden, View: FeedViewDisputed, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateHidden, NewState: VisibilityHidden},
		},
		{
			name: "live to retracted announces resolved",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityRetracted, View: FeedViewResolved, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateResolved, NewState: VisibilityRetracted},
		},
		{
			name: "hidden back to live announces reached live",
			in:   DecideInput{Cursor: cursor(VisibilityHidden, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateReachedLive, NewState: VisibilityLive},
		},
		{
			name: "update exactly one hour old allows the next one",
			in:   DecideInput{Cursor: cursor(VisibilityLive, updatesPtr(now.Add(-UpdateReportGap)), nil), Visibility: VisibilityHidden, View: FeedViewDisputed, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateHidden, NewState: VisibilityHidden},
		},
		{
			name: "same change inside the hour writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &recent, nil), Visibility: VisibilityHidden, View: FeedViewDisputed, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "a retraction inside the hour waits too",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &recent, nil), Visibility: VisibilityRetracted, View: FeedViewResolved, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "provisional never moves the announced state",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityProvisional, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "provisional from a hidden cursor writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityHidden, &longAgo, nil), Visibility: VisibilityProvisional, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "live then provisional then live again collapses",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: farExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "no change and nothing fading writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityHidden, &longAgo, nil), Visibility: VisibilityHidden, View: FeedViewDisputed, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "active report within an hour of expiring fades",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideFading, Kind: UpdateFading, NewState: VisibilityLive},
		},
		{
			name: "fading with no earlier update at all",
			in:   DecideInput{Cursor: cursor(VisibilityLive, nil, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideFading, Kind: UpdateFading, NewState: VisibilityLive},
		},
		{
			name: "expiry exactly one hour away fades",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: now.Add(FadingLead), Now: now},
			want: Decision{Action: DecideFading, Kind: UpdateFading, NewState: VisibilityLive},
		},
		{
			name: "expiry more than an hour away does not fade",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: now.Add(FadingLead + time.Second), Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "fading already sent for this expiry writes nothing",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, updatesPtr(soonExpiry)), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "a changed expiry fades again after the gap",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, updatesPtr(now.Add(30*time.Minute))), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideFading, Kind: UpdateFading, NewState: VisibilityLive},
		},
		{
			name: "fading inside the hour gap waits",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &recent, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "expired report never fades",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewPast, ExpiresAt: now, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "active view with an expiry already past never fades",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: now.Add(-time.Minute), Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "disputed view never fades",
			in:   DecideInput{Cursor: cursor(VisibilityHidden, &longAgo, nil), Visibility: VisibilityHidden, View: FeedViewDisputed, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "resolved view never fades",
			in:   DecideInput{Cursor: cursor(VisibilityRetracted, &longAgo, nil), Visibility: VisibilityRetracted, View: FeedViewResolved, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "past view never fades",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewPast, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideNothing},
		},
		{
			name: "a state event wins over fading in the same call",
			in:   DecideInput{Cursor: cursor(VisibilityProvisional, &longAgo, nil), Visibility: VisibilityLive, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideEvent, Kind: UpdateReachedLive, NewState: VisibilityLive},
		},
		{
			name: "provisional report within an hour of expiring fades without moving the state",
			in:   DecideInput{Cursor: cursor(VisibilityLive, &longAgo, nil), Visibility: VisibilityProvisional, View: FeedViewActive, ExpiresAt: soonExpiry, Now: now},
			want: Decision{Action: DecideFading, Kind: UpdateFading, NewState: VisibilityLive},
		},
	}
	if len(cases) < 16 {
		t.Fatalf("only %d cases", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DecideUpdate(c.in)
			if got != c.want {
				t.Errorf("DecideUpdate = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestDecideUpdateMutedAndWithheldAreSilent runs every visibility and every
// cursor shape through the two rules that must write nothing (D-11, Pitfall 9).
func TestDecideUpdateMutedAndWithheldAreSilent(t *testing.T) {
	now := updatesNow
	cursors := []*UpdateCursor{
		nil,
		{LastState: VisibilityLive, AdoptedAt: now.Add(-time.Hour)},
		{LastState: VisibilityProvisional, AdoptedAt: now.Add(-time.Hour), LastUpdateAt: updatesPtr(now.Add(-3 * time.Hour))},
	}
	for _, vis := range Visibilities {
		for _, view := range append([]FeedView{FeedViewNone}, FeedViews...) {
			for _, cur := range cursors {
				for _, own := range []*time.Time{nil, updatesPtr(now.Add(-time.Minute))} {
					in := DecideInput{Cursor: cur, Visibility: vis, View: view, ExpiresAt: now.Add(30 * time.Minute), OwnLatestVoteAt: own, Muted: true, Now: now}
					if got := DecideUpdate(in); got.Action != DecideNothing {
						t.Errorf("muted: vis=%q view=%q cursor=%v: %+v", vis, view, cur, got)
					}
					if vis == VisibilityWithheld {
						in.Muted = false
						if got := DecideUpdate(in); got.Action != DecideNothing {
							t.Errorf("withheld: view=%q cursor=%v: %+v", view, cur, got)
						}
					}
				}
			}
		}
	}
}

func TestMayNotifyAboutReport(t *testing.T) {
	future := updatesNow.Add(time.Hour)
	cases := []struct {
		name    string
		kind    UpdateKind
		vis     Visibility
		expires time.Time
		want    bool
	}{
		{name: "reliable note, live", kind: UpdateReliableNearby, vis: VisibilityLive, expires: future, want: true},
		{name: "reliable note, provisional (at posting)", kind: UpdateReliableNearby, vis: VisibilityProvisional, expires: future, want: true},
		{name: "reliable note, hidden", kind: UpdateReliableNearby, vis: VisibilityHidden, expires: future, want: false},
		{name: "reliable note, retracted", kind: UpdateReliableNearby, vis: VisibilityRetracted, expires: future, want: false},
		{name: "reliable note, withheld", kind: UpdateReliableNearby, vis: VisibilityWithheld, expires: future, want: false},
		{name: "reliable note, live but expired at now", kind: UpdateReliableNearby, vis: VisibilityLive, expires: updatesNow, want: false},
		{name: "reliable note, provisional but expired", kind: UpdateReliableNearby, vis: VisibilityProvisional, expires: updatesNow.Add(-time.Minute), want: false},
		{name: "reached live, live", kind: UpdateReachedLive, vis: VisibilityLive, expires: future, want: true},
		{name: "reached live, provisional", kind: UpdateReachedLive, vis: VisibilityProvisional, expires: future, want: false},
		{name: "fading, provisional", kind: UpdateFading, vis: VisibilityProvisional, expires: future, want: false},
		{name: "hidden kind, hidden", kind: UpdateHidden, vis: VisibilityHidden, expires: future, want: false},
		{name: "resolved kind, retracted", kind: UpdateResolved, vis: VisibilityRetracted, expires: future, want: false},
		{name: "more, live", kind: UpdateMore, vis: VisibilityLive, expires: future, want: true},
		{name: "unknown kind, provisional", kind: "comment", vis: VisibilityProvisional, expires: future, want: false},
		{name: "unknown visibility", kind: UpdateReliableNearby, vis: "mystery", expires: future, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MayNotifyAboutReport(c.kind, c.vis, c.expires, updatesNow); got != c.want {
				t.Errorf("MayNotifyAboutReport(%q, %q) = %v, want %v", c.kind, c.vis, got, c.want)
			}
		})
	}
}

// TestMayNotifyMatchesResolve compares the predicate with the resolver over a
// severity, category, moderation and tally grid, so a second copy of the
// critical rule cannot appear (T-03.2-05).
func TestMayNotifyMatchesResolve(t *testing.T) {
	now := updatesNow
	tallies := []VoteTally{
		{},
		{ConfirmCells: 1},
		{ConfirmCells: 2},
		{ConfirmCells: 2, DisputeCells: 2},
		{DisputeCells: 2},
		{ConfirmCells: 1, DisputeCells: 2},
		{ConfirmCells: 2, ReporterResolved: true},
		{ConfirmCells: 2, ResolveCells: 2},
		{DisputeCells: 2, ReporterResolved: true, ReporterReopened: true},
	}
	expiries := []time.Time{now.Add(time.Hour), now, now.Add(-time.Hour)}
	categories := []Category{CategoryFlood, CategoryRescueNeeded, CategoryTraffic}
	severities := Severities

	sawAlertOnly, sawBoth, sawWithheldSerious, sawWithheldPlain := false, false, false, false
	for _, sev := range severities {
		for _, cat := range categories {
			for _, mod := range ModerationStates {
				meta := ReportMeta{Severity: sev, Category: cat, Moderation: mod}
				for _, tally := range tallies {
					vis, _ := Resolve(meta, tally, now)
					for _, exp := range expiries {
						unexpired := exp.After(now)
						wantAlert := (vis == VisibilityLive || vis == VisibilityProvisional) && unexpired
						wantOther := vis == VisibilityLive && unexpired
						if got := MayNotifyAboutReport(UpdateReliableNearby, vis, exp, now); got != wantAlert {
							t.Errorf("reliable note: meta=%+v tally=%+v vis=%q exp=%v: got %v, want %v", meta, tally, vis, exp, got, wantAlert)
						}
						for _, k := range UpdateKinds {
							if k == UpdateReliableNearby {
								continue
							}
							if got := MayNotifyAboutReport(k, vis, exp, now); got != wantOther {
								t.Errorf("kind %q: meta=%+v tally=%+v vis=%q exp=%v: got %v, want %v", k, meta, tally, vis, exp, got, wantOther)
							}
						}
						if wantAlert && !wantOther {
							sawAlertOnly = true
						}
						if wantAlert && wantOther {
							sawBoth = true
						}
						if vis == VisibilityWithheld {
							for _, k := range UpdateKinds {
								if MayNotifyAboutReport(k, vis, exp, now) {
									t.Errorf("withheld notifies under %q: meta=%+v", k, meta)
								}
							}
							if meta.Serious() {
								sawWithheldSerious = true
							} else {
								sawWithheldPlain = true
							}
						}
					}
				}
			}
		}
	}
	if !sawAlertOnly || !sawBoth {
		t.Errorf("grid did not reach both the provisional only and the live cases (alertOnly=%v both=%v)", sawAlertOnly, sawBoth)
	}
	if !sawWithheldPlain || !sawWithheldSerious {
		t.Errorf("withheld coverage wrong: plain=%v serious=%v (a removed serious report and a pending plain one are both Withheld)", sawWithheldPlain, sawWithheldSerious)
	}

	// A pending serious report is published while its check retries, so it is
	// Live and notifies; a pending non serious one is Withheld and never does.
	serious := ReportMeta{Severity: SeverityCritical, Category: CategoryFlood, Moderation: ModerationPending}
	if vis, _ := Resolve(serious, VoteTally{}, now); vis != VisibilityLive || !MayNotifyAboutReport(UpdateReachedLive, vis, now.Add(time.Hour), now) {
		t.Errorf("pending serious report: vis=%q, want live and notifying", vis)
	}
	plain := ReportMeta{Severity: SeverityLow, Category: CategoryFlood, Moderation: ModerationPending}
	if vis, _ := Resolve(plain, VoteTally{ConfirmCells: 5}, now); vis != VisibilityWithheld {
		t.Errorf("pending plain report: vis=%q, want withheld", vis)
	}
}
