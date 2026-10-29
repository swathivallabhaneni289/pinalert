package demo

// seed.go fills a new Demo world with the sample a visitor opens on: a moment
// of the 2024 Vijayawada floods along the Budameru (31 August to 1 September
// 2024). The place names are real, the reports are made up, and no real person,
// phone number or street address appears (D-02, D-05).
//
// Every state a visitor sees is derived, never painted (TRUST-02): each report is
// posted through the real ReportService.SubmitChecked with the world clock set
// into the past, and each confirmation or dispute is cast through the real
// VotingService.CastVote from a synthetic voter at its own precision 7 cell. So
// Confirmed, Disputed and Ended come from the resolver, not from a field.
//
// Coordinates. Research did not verify them, so each place was resolved once by
// the executor of plan 05-06 through the public Nominatim search
// (https://nominatim.openstreetmap.org/search, format=json, limit=1,
// countrycodes=in, the place name plus Vijayawada), on 2026-10-29, with a
// User-Agent of Pinalert and the repository URL. Ten requests were made, one
// every few seconds. Values are rounded to 4 decimals.
//
//	Singh Nagar          16.5424  80.6386  The query "Singh Nagar, Vijayawada" returned a primary
//	                                       road (Vijayawada - Nuzvidu Road) in the Singh Nagar and
//	                                       Ayodhya Nagar area, the closest hit to the place. It is
//	                                       the pretend position. A second query (Singh Nagar, NTR
//	                                       District) returned the Ajit Singh Nagar node instead.
//	Ajit Singh Nagar     16.5395  80.6405  OSM neighbourhood node.
//	Rajarajeswari Peta   16.5316  80.6230  OSM neighbourhood node.
//	Payakapuram          16.5489  80.6496  OSM neighbourhood node.
//	Krishnalanka         16.5017  80.6271  The only hit was a health centre point in Krishnalanka,
//	                                       used here as a point inside the neighbourhood.
//	Ibrahimpatnam        16.5940  80.5225  OSM town node. The query with Vijayawada returned a
//	                                       school, so the town was resolved by a second query
//	                                       (Ibrahimpatnam, NTR District, Andhra Pradesh).
//
// Two names did not give a usable point, so no report sits there. Bhupesh Nagar
// returned nothing. The Budameru returned a river relation centre near Kankipadu
// (16.5189, 80.7594), about 12 km east and in another district, which would
// mislead. The Budameru is therefore named in the report text only, and the
// reports that speak of its bank sit at the resolved Ajit Singh Nagar point,
// which the stream runs beside.
//
// Several reports share one resolved place. They are spread by invented offsets
// (metres and a bearing, written in the table below) so the pins do not sit on
// top of each other. The offsets are not claims about any street.

import (
	"context"
	"fmt"
	"time"

	"pinalert/internal/service"
)

// PretendLat and PretendLon are the fixed position every Demo vote is sent from
// (research G5 gotcha 1): a resolved point in Singh Nagar.
const PretendLat = 16.5424

// PretendLon is the longitude of the pretend position.
const PretendLon = 80.6386

// PretendPlace names the pretend position for the line under the map.
const PretendPlace = "Singh Nagar"

// seedPlace is a resolved place (see the header).
type seedPlace struct {
	name     string
	lat, lon float64
}

var (
	placeSingh   = seedPlace{"Singh Nagar", PretendLat, PretendLon}
	placeAjit    = seedPlace{"Ajit Singh Nagar", 16.5395, 80.6405}
	placeRaja    = seedPlace{"Rajarajeswari Peta", 16.5316, 80.6230}
	placePayak   = seedPlace{"Payakapuram", 16.5489, 80.6496}
	placeKrishna = seedPlace{"Krishnalanka", 16.5017, 80.6271}
	placeIbrahim = seedPlace{"Ibrahimpatnam", 16.5940, 80.5225}
)

// seedReport is one made up report. confirms and disputes are the number of seed
// voters who vote on it: the table holds reports and votes and nothing else, so
// every state comes from the resolver.
type seedReport struct {
	place           seedPlace
	metres, bearing float64 // invented offset from the place
	ago             time.Duration
	category        service.Category
	severity        service.Severity
	description     string
	capacity        service.CapacityStatus // shelter reports only
	happening       service.Happening      // critical reports only, optional
	confirms        int
	disputes        int
}

// seedReports is the sample. The first eleven lie within 1 km of the pretend
// position and the last nine beyond it. The Ended report is posted more than
// ExpiryDuration(medium) before the world start, so it reads as ended inside the
// Past window. The Disputed report is a medium report with three disputes and no
// confirmation, which the resolver hides. Nothing is Retracted.
var seedReports = []seedReport{
	// Near the pretend position (within 1 km).
	{placeSingh, 120, 45, 20 * time.Minute, service.CategoryFlood, service.SeverityCritical,
		"Water is chest deep on the main road and still rising. People are moving to upper floors.",
		"", service.HappeningWaterRising, 3, 0},
	{placeAjit, 200, 90, 35 * time.Minute, service.CategoryRoadBlocked, service.SeverityMedium,
		"The lane beside the Budameru bank is under knee deep water. Two wheelers cannot get through.",
		"", "", 2, 0},
	{placeAjit, 100, 200, 15 * time.Minute, service.CategoryShelterOpen, service.SeverityLow,
		"Community hall is open as a shelter. Mats and drinking water are available and there is room for more families.",
		service.CapacityAvailable, "", 1, 0},
	{placeSingh, 300, 270, 50 * time.Minute, service.CategoryPowerOutage, service.SeverityMedium,
		"Power has been off since the water reached the transformer. Nothing is running on this street.",
		"", "", 1, 0},
	{placeAjit, 250, 135, 10 * time.Minute, service.CategoryStormCyclone, service.SeverityMedium,
		"Heavy rain and strong gusts again over the Budameru side. Branches are coming down.",
		"", "", 0, 0},
	{placeSingh, 400, 0, 60 * time.Minute, service.CategoryTraffic, service.SeverityLow,
		"Traffic is crawling at the junction because water covers one lane. Use the higher road if you can.",
		"", "", 1, 0},
	{placeAjit, 300, 315, 25 * time.Minute, service.CategoryFire, service.SeverityMedium,
		"Smoke from a flooded electrical box near the lane. Please stay away from the wet wires.",
		"", "", 0, 0},
	{placeSingh, 200, 180, 45 * time.Minute, service.CategoryFlood, service.SeverityLow,
		"Water is ankle deep in the lane and slowly going down.",
		"", "", 1, 0},
	{placeAjit, 150, 45, 40 * time.Minute, service.CategoryOther, service.SeverityLow,
		"A drinking water tanker is parked at the corner and people are queueing calmly.",
		"", "", 1, 0},
	// The Disputed report: three people at different spots say it is not true.
	{placeSingh, 350, 90, 30 * time.Minute, service.CategoryRoadBlocked, service.SeverityMedium,
		"The main bridge is fully blocked and nobody can cross.",
		"", "", 0, 3},
	// The Ended report: posted nine hours before the world start, so its eight
	// hour life ended an hour before it.
	{placeAjit, 80, 270, 9 * time.Hour, service.CategoryFlood, service.SeverityMedium,
		"Water crossed the lane early in the day and has since gone down.",
		"", "", 2, 0},

	// Farther than 1 km from the pretend position.
	{placeRaja, 100, 0, 18 * time.Minute, service.CategoryFlood, service.SeverityCritical,
		"Water is above waist height in the narrow streets. Some families are on rooftops waiting for boats.",
		"", "", 3, 0},
	{placeRaja, 250, 90, 55 * time.Minute, service.CategoryShelterOpen, service.SeverityMedium,
		"A school building is taking in families from the flooded lanes. Beds are limited.",
		service.CapacityLimited, "", 2, 0},
	{placeRaja, 200, 225, 28 * time.Minute, service.CategoryPowerOutage, service.SeverityLow,
		"Street lights are off after the supply was cut as a safety step.",
		"", "", 1, 0},
	{placePayak, 80, 45, 33 * time.Minute, service.CategoryTraffic, service.SeverityMedium,
		"A long queue of vehicles is waiting at the underpass because of water. Take another route.",
		"", "", 1, 0},
	{placePayak, 60, 0, 12 * time.Minute, service.CategoryEarthquake, service.SeverityLow,
		"A short tremor was felt by a few people. No damage seen so far.",
		"", "", 0, 0},
	{placeKrishna, 100, 270, 70 * time.Minute, service.CategoryFlood, service.SeverityMedium,
		"Water has entered the low houses along the canal bank and people are moving belongings upstairs.",
		"", "", 1, 0},
	{placeKrishna, 300, 180, 22 * time.Minute, service.CategoryRoadBlocked, service.SeverityCritical,
		"The embankment road has washed out in one place. Do not drive here.",
		"", "", 2, 0},
	{placeKrishna, 200, 90, 80 * time.Minute, service.CategoryStormCyclone, service.SeverityLow,
		"Wind has dropped but rain continues. Boats are being used in the lower lanes.",
		"", "", 1, 0},
	{placeIbrahim, 150, 0, 65 * time.Minute, service.CategoryOther, service.SeverityMedium,
		"Relief food packets are being given out near the town centre.",
		"", "", 0, 0},
}

// seedVoteSpots are where the seed voters of one report stand, as metres and a
// bearing from that report: each within 1 km of it, at least 300 m from it and
// from each other, so each lands in its own precision 7 cell (about 150 m).
var seedVoteSpots = []struct{ metres, bearing float64 }{
	{300, 0}, {450, 90}, {600, 180}, {750, 270}, {400, 45}, {550, 225},
}

// seedVoteGap is how long after the report, or the previous vote, a seed vote is
// cast on the world clock.
const seedVoteGap = 2 * time.Minute

// Seed fills w with the sample. It runs the real services with the world clock
// set into the past and restores the clock to the wall clock before it returns.
// An error from a real service is returned with the report it concerned: the
// table is checked by tests, so an error here is a bug in the table.
func Seed(w *World) error {
	defer w.SetClockOffset(0)
	ctx := context.Background()
	voters := int64(0)

	for n, sr := range seedReports {
		lat, lon := offsetPoint(sr.place.lat, sr.place.lon, sr.metres, sr.bearing)
		reporter := SeedReporterAccountBase + int64(n)

		in := service.SubmitInput{
			Category:    sr.category,
			Severity:    sr.severity,
			Description: sr.description,
			Latitude:    lat,
			Longitude:   lon,
		}
		if sr.capacity != "" {
			c := sr.capacity
			in.ShelterCapacityStatus = &c
		}
		if sr.happening != "" {
			h := sr.happening
			in.Happening = &h
		}

		w.SetClockOffset(-sr.ago)
		res, err := w.Reports.SubmitChecked(ctx, w.SessionFor(reporter), reporter, in)
		if err != nil {
			return fmt.Errorf("demo seed: report %d (%s): %w", n, sr.place.name, err)
		}

		cast := 0
		vote := func(value service.VoteValue) error {
			spot := seedVoteSpots[cast%len(seedVoteSpots)]
			vLat, vLon := offsetPoint(lat, lon, spot.metres, spot.bearing)
			cast++
			w.SetClockOffset(-sr.ago + time.Duration(cast)*seedVoteGap)
			_, err := w.Votes.CastVote(ctx, service.CastVoteInput{
				ReportID:  res.Report.ID,
				AccountID: SeedVoterAccountBase + voters,
				Kind:      service.VoteKindContent,
				Value:     value,
				Latitude:  vLat,
				Longitude: vLon,
			})
			voters++
			return err
		}
		for i := 0; i < sr.confirms; i++ {
			if err := vote(service.VoteConfirm); err != nil {
				return fmt.Errorf("demo seed: confirm %d on report %d (%s): %w", i+1, n, sr.place.name, err)
			}
		}
		for i := 0; i < sr.disputes; i++ {
			if err := vote(service.VoteDispute); err != nil {
				return fmt.Errorf("demo seed: dispute %d on report %d (%s): %w", i+1, n, sr.place.name, err)
			}
		}
	}
	return nil
}

// SeedSetup is Seed shaped for WithWorldSetup. It logs nothing: a failure of the
// table is caught by the seed tests, never at a visitor's request.
func SeedSetup(w *World) {
	_ = Seed(w)
}

// SampleKeyPrefix starts the Key of every sample official alert.
const SampleKeyPrefix = "sample-"

// SampleAlert is an invented official alert for the Demo's Official alerts page
// and banner (D-05). It is plain data: plan 05-19 turns it into the stored alert
// values with times relative to the moment it is read. Every value is marked as
// a sample, and the text is invented, not copied from any issuer's message. The
// issuer is a made up "Sample ..." name, never a real one, so a cropped row
// cannot pass for a real warning (IN-S08 of the Phase 5 review).
type SampleAlert struct {
	// Key is unique and starts with SampleKeyPrefix.
	Key string
	// Sample is true on every entry.
	Sample bool
	// Severity is the CAP word: Extreme, Severe or Moderate here.
	Severity   string
	Event      string
	HeadlineEN string
	Issuer     string
	// Area is the district name (Tier "district") or the state name (Tier
	// "state") the alert is placed on.
	Area string
	// Tier is "district" or "state", the placement tiers of the official feed.
	Tier string
	// IssuedAgo is how long before the read the alert was issued, and EndsIn how
	// long after it the alert runs.
	IssuedAgo, EndsIn time.Duration
	Description       string
	Instruction       string
}

// SampleAlerts are the Demo's official alerts: a Severe and an Extreme for the
// district the pretend position falls in (NTR) and a Moderate across Andhra
// Pradesh. Their text is invented.
var SampleAlerts = []SampleAlert{
	{
		Key:        SampleKeyPrefix + "extreme-flood-ntr",
		Sample:     true,
		Severity:   "Extreme",
		Event:      "Flood warning",
		HeadlineEN: "Sample: very high water expected along the Budameru and low lying parts of the city",
		Issuer:     "Sample river office",
		Area:       "NTR",
		Tier:       "district",
		IssuedAgo:  40 * time.Minute,
		EndsIn:     12 * time.Hour,
		Description: "This is a sample alert written for the Demo. Water levels along the Budameru are expected to stay very high " +
			"through the day, and low lying lanes may be cut off.",
		Instruction: "Move to higher ground early and keep to roads that are open.",
	},
	{
		Key:        SampleKeyPrefix + "severe-rain-ntr",
		Sample:     true,
		Severity:   "Severe",
		Event:      "Heavy rain warning",
		HeadlineEN: "Sample: heavy to very heavy rain likely over the district for the next day",
		Issuer:     "Sample weather office",
		Area:       "NTR",
		Tier:       "district",
		IssuedAgo:  3 * time.Hour,
		EndsIn:     21 * time.Hour,
		Description: "This is a sample alert written for the Demo. Spells of heavy rain are likely, with a chance of waterlogging " +
			"in low areas.",
		Instruction: "Avoid travel through water and keep phones charged.",
	},
	{
		Key:        SampleKeyPrefix + "moderate-lightning-ap",
		Sample:     true,
		Severity:   "Moderate",
		Event:      "Lightning",
		HeadlineEN: "Sample: thunder and lightning likely at isolated places across the state",
		Issuer:     "Sample state authority",
		Area:       "Andhra Pradesh",
		Tier:       "state",
		IssuedAgo:  90 * time.Minute,
		EndsIn:     6 * time.Hour,
		Description: "This is a sample alert written for the Demo. Thunder and lightning are likely at isolated places " +
			"during the evening.",
		Instruction: "Stay indoors during lightning and keep away from trees and open fields.",
	},
}
