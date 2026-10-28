package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/official"
)

// officialUnavailableMessage is the plain sentence for an official alerts
// service that is missing or could not answer. It never says that no alert is
// in force: a failed read is not an empty list (T-34-17, ALERT-04).
const officialUnavailableMessage = "Official alerts are not available right now."

// OfficialReader is what GET /api/official/alerts needs of the official alerts
// service. *official.Service satisfies it; a test can substitute a stub.
type OfficialReader interface {
	// Alerts answers for one signed in reader. lat and lon are both set or both
	// nil.
	Alerts(ctx context.Context, accountID int64, lat, lon *float64) (official.View, error)
	// Kick starts a non blocking refresh of the feed when it is due.
	Kick()
}

// OfficialAlertsResponse is the answer of GET /api/official/alerts. The server
// authors every fact in it: the page draws it and recomputes none of it. state
// is fresh, stale or unavailable, from the last good check alone. Only a fresh
// answer with no rows in any group may be read as "no official alerts in force".
type OfficialAlertsResponse struct {
	// State is fresh (last good check within 30 minutes), stale (up to 6 hours)
	// or unavailable (never checked, or over 6 hours).
	State string `json:"state" enums:"fresh,stale,unavailable" example:"fresh"`
	// AgeSeconds is the time since the last good check by the server clock.
	// Null when the feed was never checked.
	AgeSeconds *int64 `json:"age_seconds" extensions:"x-nullable" example:"240"`
	// LastSuccessAt is the time of the last good check. Null when never checked.
	LastSuccessAt *time.Time `json:"last_success_at" extensions:"x-nullable" example:"2026-10-29T06:25:00Z"`
	// AreaKnown is false when the request carried no usable position and the
	// account has no home area; groups is then empty and that is not an
	// answer of "no alerts".
	AreaKnown bool `json:"area_known" example:"true"`
	// Groups are drawn in this order: reader, home, state, unplaced. A group
	// with no rows is left out. Empty when the state is unavailable.
	Groups []OfficialGroupResponse `json:"groups"`
	// Count is the number of rows across all groups; a grouped row counts once.
	Count int `json:"count" example:"3"`
	// DistrictsLine names the districts the page covers, for example
	// "Krishna, NTR, Guntur and 4 more".
	DistrictsLine string `json:"districts_line" example:"Krishna district"`
	// Banner is the Nearby banner pick, or null. Only for a fresh answer and
	// the reader's own area.
	Banner *OfficialBannerResponse `json:"banner" extensions:"x-nullable"`
}

// OfficialGroupResponse is one labelled section of rows.
type OfficialGroupResponse struct {
	Kind  string                `json:"kind" enums:"reader,home,state,unplaced" example:"reader"`
	Label string                `json:"label" example:"Where you are: Krishna district"`
	Rows  []OfficialRowResponse `json:"rows"`
}

// OfficialRowResponse is one alert, or a run of near identical alerts shown as
// the newest with grouped_count above 1.
type OfficialRowResponse struct {
	ID        int64  `json:"id" example:"41"`
	Source    string `json:"source" enums:"sachet,gdacs" example:"sachet"`
	Level     string `json:"level" enums:"extreme,severe,moderate,minor,unknown" example:"severe"`
	LevelWord string `json:"level_word" example:"Severe"`
	// Badge is "Official" for NDMA SACHET, "GDACS" for GDACS, and empty for a
	// Demo sample.
	Badge string `json:"badge" example:"Official"`
	Demo  bool   `json:"demo" example:"false"`
	// Headline is the English headline when the feed has one, else the original
	// language headline (original_language_only is then true).
	Headline             string `json:"headline" example:"Heavy rain expected"`
	OriginalLanguageOnly bool   `json:"original_language_only" example:"false"`
	AreaText             string `json:"area_text" example:"6 mandals"`
	Issuer               string `json:"issuer" example:"IMD Chennai"`
	IssuedAt             string `json:"issued_at" example:"2026-10-29T05:40:00Z"`
	// ExpiresAt is the end time; for a grouped row, the latest end.
	ExpiresAt       string                  `json:"expires_at" example:"2026-10-29T14:35:00Z"`
	ExpiresInferred bool                    `json:"expires_inferred" example:"false"`
	GroupedCount    int                     `json:"grouped_count" example:"1"`
	Details         OfficialDetailsResponse `json:"details"`
}

// OfficialDetailsResponse is the disclosure panel of a row. Every part is empty
// when the feed did not carry it. All of it is feed text and must be drawn with
// textContent only.
type OfficialDetailsResponse struct {
	Description         string `json:"description"`
	Instruction         string `json:"instruction"`
	OriginalHeadline    string `json:"original_headline"`
	OriginalDescription string `json:"original_description"`
	// OriginalLangLabel is a language name such as Telugu, or the feed's own
	// label when it is not known. OriginalLangCode is a BCP 47 code only for
	// the known ones, else empty.
	OriginalLangLabel string `json:"original_lang_label" example:"Telugu"`
	OriginalLangCode  string `json:"original_lang_code" example:"te"`
	Areas             string `json:"areas"`
	SourceLine        string `json:"source_line" example:"NDMA SACHET"`
}

// OfficialBannerResponse is the single Nearby banner line.
type OfficialBannerResponse struct {
	Level       string `json:"level" enums:"severe,extreme" example:"extreme"`
	LevelWord   string `json:"level_word" example:"Extreme"`
	EventPhrase string `json:"event_phrase" example:"Cyclone warning"`
	Area        string `json:"area" example:"Krishna district"`
	// More is how many other eligible rows there are.
	More    int   `json:"more" example:"2"`
	AlertID int64 `json:"alert_id" example:"41"`
	// Demo is true on the Demo's sample alert (plan 05-19) and false on every
	// real one. The banner then draws the Demo tag.
	Demo bool `json:"demo" example:"false"`
}

// OfficialAlerts handles GET /api/official/alerts.
//
// @Summary Official alerts for the reader's area
// @Description Returns the official alerts (NDMA SACHET) that cover the reader and whether that answer can be trusted right now.
// @Description The optional lat and lon (both or neither) are the position Nearby is using. The server turns them into a
// @Description district at once and keeps no reader location: it is neither stored nor logged. The home area saved on the
// @Description account is read in the same way and is never returned.
// @Description The server authors every fact in the answer: state (fresh within 30 minutes of the last good check, stale up to
// @Description 6 hours, unavailable after that or never), age_seconds from the server clock, the group labels, the row count,
// @Description the districts line, each row's level, authority badge, issuer and times, and the banner pick. The page recomputes none of it.
// @Description Groups come in this order: where the reader is, the home area when it is another district, state level alerts
// @Description for the state, and alerts that could not be placed. Within a group the order is Extreme, Severe, Moderate, Minor,
// @Description Unknown, newest first. Near identical alerts are one row with grouped_count.
// @Description When the state is stale, rows still inside their own expiry are sent as the last known ones and banner is null.
// @Description When it is unavailable no rows are sent. Only a fresh answer with area_known true and no rows may be read as
// @Description "no official alerts in force".
// @Description A read also starts a background refresh of the feed when the last attempt is over 10 minutes old; it never delays the answer.
// @Tags official
// @Produce json
// @Param lat query number false "Reader latitude, -90 to 90, sent together with lon"
// @Param lon query number false "Reader longitude, -180 to 180, sent together with lat"
// @Success 200 {object} OfficialAlertsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /official/alerts [get]
func OfficialAlerts(svc OfficialReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// A request with no verified account is a routing bug (a route mounted
		// outside the gated group), the same shape AlertsPage uses.
		acc, ok := account.FromContext(r.Context())
		if !ok {
			log.Printf("handlers: OfficialAlerts: no verified account in request context: the gate should have made this impossible")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Both or neither, range checked, before anything else is done.
		point, ok := parsePlaceCentre(w, r)
		if !ok {
			return
		}

		if svc == nil {
			// A Deps{} literal that omits the dependency must degrade to a plain
			// message, not a panic.
			log.Printf("handlers: OfficialAlerts: no official alerts service")
			writeFieldError(w, http.StatusServiceUnavailable, "official", officialUnavailableMessage)
			return
		}

		var lat, lon *float64
		if point != nil {
			lat, lon = &point.Lat, &point.Lon
		}
		view, err := svc.Alerts(r.Context(), acc.ID, lat, lon)
		if err != nil {
			// The class only: an error text could carry a query argument.
			log.Printf("handlers: OfficialAlerts: read failed (%s)", officialErrorClass(err))
			writeFieldError(w, http.StatusServiceUnavailable, "official", officialUnavailableMessage)
			return
		}

		// The refresh is started after the answer is built and never waited for.
		svc.Kick()

		// Set before writeJSON, since writeJSON calls WriteHeader.
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, officialViewToResponse(view))
	}
}

// officialErrorClass names a failed read in a few fixed words for the log.
func officialErrorClass(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	}
	return "database"
}

// officialViewToResponse is the one conversion from the view to the wire shape.
// The Demo (plan 05-19) uses it too, so a sample answer has the real shape.
// Slices are never nil, so the JSON holds [] and not null.
func officialViewToResponse(v official.View) OfficialAlertsResponse {
	out := OfficialAlertsResponse{
		State:         string(v.State),
		AgeSeconds:    v.AgeSeconds,
		AreaKnown:     v.AreaKnown,
		Groups:        make([]OfficialGroupResponse, 0, len(v.Groups)),
		Count:         v.Count,
		DistrictsLine: v.DistrictsLine,
	}
	if v.LastSuccessAt != nil {
		t := v.LastSuccessAt.UTC()
		out.LastSuccessAt = &t
	}
	for _, g := range v.Groups {
		gr := OfficialGroupResponse{Kind: g.Kind, Label: g.Label, Rows: make([]OfficialRowResponse, 0, len(g.Rows))}
		for _, row := range g.Rows {
			gr.Rows = append(gr.Rows, OfficialRowResponse{
				ID:                   row.ID,
				Source:               row.Source,
				Level:                row.Level,
				LevelWord:            row.LevelWord,
				Badge:                row.Badge,
				Demo:                 row.Demo,
				Headline:             row.Headline,
				OriginalLanguageOnly: row.OriginalLanguageOnly,
				AreaText:             row.AreaText,
				Issuer:               row.Issuer,
				IssuedAt:             row.IssuedAt.UTC().Format(time.RFC3339),
				ExpiresAt:            row.ExpiresAt.UTC().Format(time.RFC3339),
				ExpiresInferred:      row.ExpiresInferred,
				GroupedCount:         row.GroupedCount,
				Details: OfficialDetailsResponse{
					Description:         row.Details.Description,
					Instruction:         row.Details.Instruction,
					OriginalHeadline:    row.Details.OriginalHeadline,
					OriginalDescription: row.Details.OriginalDescription,
					OriginalLangLabel:   row.Details.OriginalLangLabel,
					OriginalLangCode:    row.Details.OriginalLangCode,
					Areas:               row.Details.Areas,
					SourceLine:          row.Details.SourceLine,
				},
			})
		}
		out.Groups = append(out.Groups, gr)
	}
	if b := v.Banner; b != nil {
		out.Banner = &OfficialBannerResponse{
			Level:       b.Level,
			LevelWord:   b.LevelWord,
			EventPhrase: b.EventPhrase,
			Area:        b.Area,
			More:        b.More,
			AlertID:     b.AlertID,
			Demo:        b.Demo,
		}
	}
	return out
}
