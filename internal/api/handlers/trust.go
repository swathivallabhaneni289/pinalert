// trust.go is the one place the service's trust object becomes JSON. The feed,
// every vote response and, later, the Activity page all use trustToResponse,
// so they cannot drift (D-21). The handler computes nothing: every count,
// capped numeral, score and sentence comes from internal/service. The object
// carries counts, scores and server sentences only, never an account id, an
// email, a voter's place, a session id or a raw vote total.

package handlers

import "pinalert/internal/service"

// TrustResponse is the server built trust object for one report. It carries
// the place counts and panel sentences, then the two signals side by side: the
// still current signal and the reporter signal.
type TrustResponse struct {
	// ConfirmedPlaces is how many distinct places confirm the report, counting places, not votes, so several people in one place count once (D-04).
	ConfirmedPlaces int `json:"confirmed_places" example:"1"`
	// DisputedPlaces is how many distinct places dispute the report, counted the same way.
	DisputedPlaces int `json:"disputed_places" example:"0"`
	// ConfirmedText is the confirmed count as the surface shows it, from 0 to 99 or 99+ above 99, written by the server (A27).
	ConfirmedText string `json:"confirmed_text" example:"1"`
	// DisputedText is the disputed count as the surface shows it, from 0 to 99 or 99+ above 99, written by the server (A27).
	DisputedText string `json:"disputed_text" example:"0"`
	// LeadSentence is the panel's first line, empty when there is none.
	LeadSentence string `json:"lead_sentence" example:"Needs 2 confirmations to count as confirmed."`
	// ConfirmedSentence is the panel line that explains the confirmed count.
	ConfirmedSentence string `json:"confirmed_sentence" example:"1 place confirmed this. Counts places, not votes."`
	// DisputedSentence is the panel line that explains the disputed count.
	DisputedSentence string `json:"disputed_sentence" example:"No one nearby has disputed this."`
	// Currency is the still current signal: a band, the facts behind it (how many different places confirmed recently, over what window) and a whole number score from 0 to 100, or null while there is no rating. The feed and every vote response carry the same object.
	Currency CurrencyResponse `json:"currency"`
	// Reporter is the reporter signal: a tag, the facts behind it (how many earlier reports had a clear result and how many of those people nearby confirmed) and a whole number score from 0 to 100, or null while there is no rating. The feed and every vote response carry the same object.
	Reporter ReporterResponse `json:"reporter"`
}

// CurrencyResponse is the still current signal of one report.
type CurrencyResponse struct {
	// Band is the still current band, written by the server.
	Band string `json:"band" enums:"up_to_date,getting_old,needs_reconfirming,not_yet_corroborated,not_applicable" example:"up_to_date"`
	// Score is a whole number from 0 to 100, or null while there is no rating (band not_yet_corroborated or not_applicable).
	Score *int `json:"score" extensions:"x-nullable" example:"92"`
	// Label is the everyday word for the band, written by the server, empty when no word shows.
	Label string `json:"label" example:"Up to date"`
	// Sentence is the panel line for the band, written by the server, empty when no word shows.
	Sentence string `json:"sentence" example:"3 places confirmed this in the last 2 hours."`
	// ScoreSentence is the popup line that explains the score, empty when the score is null.
	ScoreSentence string `json:"score_sentence" example:"Score 92 out of 100, based on recent confirmations."`
	// RecentPlaces is how many distinct places confirmed the report inside the window.
	RecentPlaces int `json:"recent_places" example:"3"`
	// WindowMinutes is the length of the window in whole minutes.
	WindowMinutes int `json:"window_minutes" example:"120"`
	// FadeStage is fresh, aging or stale, or null for critical and rescue needed reports and for rows with no still current word. The browser takes the worse of it and its own time stage.
	FadeStage *string `json:"fade_stage" enums:"fresh,aging,stale" extensions:"x-nullable" example:"fresh"`
}

// ReporterResponse is the reporter signal of one report.
type ReporterResponse struct {
	// Tag is the reporter tag, written by the server.
	Tag string `json:"tag" enums:"reliable,mixed,unreliable,new,none" example:"reliable"`
	// Score is a whole number from 0 to 100, or null while there is no rating (tag new or none).
	Score *int `json:"score" extensions:"x-nullable" example:"86"`
	// Label is the everyday word for the tag, written by the server, empty when no word shows.
	Label string `json:"label" example:"Reliable reporter"`
	// Sentence is the panel line for the tag, written by the server, empty when no word shows.
	Sentence string `json:"sentence" example:"5 of 5 earlier reports were confirmed by people nearby."`
	// ScoreSentence is the popup line that explains the score, empty when the score is null.
	ScoreSentence string `json:"score_sentence" example:"Score 86 out of 100, based on how often their reports were confirmed."`
	// JudgedReports is how many earlier reports have a clear result.
	JudgedReports int `json:"judged_reports" example:"5"`
	// CorroboratedReports is how many of the judged reports were confirmed by people nearby.
	CorroboratedReports int `json:"corroborated_reports" example:"5"`
}

// trustToResponse copies the fields of the service's Trust one by one. It does
// no arithmetic, no capping and no string building.
func trustToResponse(t service.Trust) TrustResponse {
	return TrustResponse{
		ConfirmedPlaces:   t.ConfirmedPlaces,
		DisputedPlaces:    t.DisputedPlaces,
		ConfirmedText:     t.ConfirmedText,
		DisputedText:      t.DisputedText,
		LeadSentence:      t.LeadSentence,
		ConfirmedSentence: t.ConfirmedSentence,
		DisputedSentence:  t.DisputedSentence,
		Currency:          currencyToResponse(t.Currency),
		Reporter:          reporterToResponse(t.Reporter),
	}
}

// currencyToResponse copies the still current signal field by field.
func currencyToResponse(c service.CurrencySignal) CurrencyResponse {
	return CurrencyResponse{
		Band:          string(c.Band),
		Score:         c.Score,
		Label:         c.Label,
		Sentence:      c.Sentence,
		ScoreSentence: c.ScoreSentence,
		RecentPlaces:  c.RecentPlaces,
		WindowMinutes: c.WindowMinutes,
		FadeStage:     fadeStageToResponse(c.FadeStage),
	}
}

// reporterToResponse copies the reporter signal field by field.
func reporterToResponse(r service.ReporterSignal) ReporterResponse {
	return ReporterResponse{
		Tag:                 string(r.Tag),
		Score:               r.Score,
		Label:               r.Label,
		Sentence:            r.Sentence,
		ScoreSentence:       r.ScoreSentence,
		JudgedReports:       r.JudgedReports,
		CorroboratedReports: r.CorroboratedReports,
	}
}

// fadeStageToResponse turns the optional fade stage into an optional string.
func fadeStageToResponse(f *service.FadeStage) *string {
	if f == nil {
		return nil
	}
	s := string(*f)
	return &s
}
