package demo

// alerts.go turns the Demo's invented official alerts (SampleAlerts, seed.go)
// into the answer of the real Official alerts view (plan 05-19, D-05, D-06,
// ROBUST-06). The sample alerts are stored-alert values placed on the pretend
// position's district and state, and the same pure view the real page uses
// builds the groups, the rows and the Nearby banner from them. So the Demo's
// Official alerts page and its banner are drawn by the real renderer from a
// real view, only the source of the alerts differs, and every row, group and the
// banner carry the Demo flag, no authority badge and no issuer channel.

import (
	"time"

	"pinalert/internal/areas"
	"pinalert/internal/official"
)

// SampleView builds the Demo's Official alerts view as of now. The feed is
// fresh and was checked this moment (age 0), the reader is the pretend
// position's district, and the sample alerts are placed on it: the district tier
// on the district, the state tier on its state. Issue and end times are relative
// to now, so a sample is always in force. w names the visit the view is for; the
// sample is the same for every visit, so a nil world is accepted, and a zero now
// takes the world's clock. A pretend position that falls in no district gives a
// view with the area unknown rather than a guess.
func SampleView(w *World, now time.Time) official.View {
	if now.IsZero() && w != nil {
		now = w.Now()
	}
	checked := now.UTC()
	in := official.ViewInput{
		Now:    now,
		Status: official.FeedStatus{Feed: official.FeedSachet, LastSuccessAt: &checked},
		Demo:   true,
	}

	if district, ok := areas.Default().Locate(PretendLat, PretendLon); ok {
		in.Reader = official.AreaOfDistrict(district)
		in.Alerts = sampleAlertsOn(district, in.Reader, now)
	}
	return official.BuildView(in)
}

// sampleAlertsOn places the sample alerts on a district and its state.
func sampleAlertsOn(district areas.District, reader *official.Area, now time.Time) []official.StoredAlert {
	alerts := make([]official.StoredAlert, 0, len(SampleAlerts))
	for i, s := range SampleAlerts {
		a := official.StoredAlert{
			ID:            int64(i + 1),
			Sender:        s.Issuer,
			Issuer:        s.Issuer,
			Event:         s.Event,
			Severity:      s.Severity,
			SentAt:        now.Add(-s.IssuedAgo).UTC(),
			ExpiresAt:     now.Add(s.EndsIn).UTC(),
			HeadlineEN:    s.HeadlineEN,
			DescriptionEN: s.Description,
			Instruction:   s.Instruction,
			Tier:          official.Tier(s.Tier),
		}
		switch a.Tier {
		case official.TierDistrict:
			a.DistrictCodes = []int{district.Code}
			a.DistrictNames = []string{district.Name}
			a.StateCodes = []int{district.StateCode}
			a.AreaDesc = reader.DistrictName + " district"
		default:
			a.StateCodes = []int{district.StateCode}
			a.AreaDesc = reader.StateName
		}
		alerts = append(alerts, a)
	}
	return alerts
}
