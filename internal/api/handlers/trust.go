// trust.go is the one place the service's trust object becomes JSON. The feed,
// every vote response and, later, the Activity page all use trustToResponse,
// so they cannot drift (D-21). The handler computes nothing: every count,
// capped numeral and sentence comes from internal/service. The object carries
// counts and server sentences only, never an account id, an email, a voter's
// place, a session id or a raw vote total.

package handlers

import "pinalert/internal/service"

// TrustResponse is the server built trust object for one report.
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
	LeadSentence string `json:"lead_sentence" example:"Needs 2 confirmations before this counts as confirmed."`
	// ConfirmedSentence is the panel line that explains the confirmed count.
	ConfirmedSentence string `json:"confirmed_sentence" example:"1 person in 1 place confirmed this. Counts separate places, not votes."`
	// DisputedSentence is the panel line that explains the disputed count.
	DisputedSentence string `json:"disputed_sentence" example:"No one nearby has disputed this. Counts separate places, not votes."`
}

// trustToResponse copies the seven fields of the service's Trust one by one.
// It does no arithmetic, no capping and no string building.
func trustToResponse(t service.Trust) TrustResponse {
	return TrustResponse{
		ConfirmedPlaces:   t.ConfirmedPlaces,
		DisputedPlaces:    t.DisputedPlaces,
		ConfirmedText:     t.ConfirmedText,
		DisputedText:      t.DisputedText,
		LeadSentence:      t.LeadSentence,
		ConfirmedSentence: t.ConfirmedSentence,
		DisputedSentence:  t.DisputedSentence,
	}
}
