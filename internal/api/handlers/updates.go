// updates.go is the HTTP surface of in app updates and report mutes (Phase 3.2,
// plan 07): the bell's poll, the read mark, the mute control and the muted list
// of Settings. Every route is in the gated group, so the account is the one the
// gate resolved and no request names an account (DEC-L: a cross site POST
// carries no session cookie, so no CSRF token is needed). Every id comes from
// the gate. No request carries a coordinate and no response carries one, an
// email, a session or account id, a voter cell or a description. Every rule
// lives in service.UpdatesService and service.MuteService; the words of a row are
// all written on the server, so the browser composes no sentence.

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pinalert/internal/account"
	"pinalert/internal/service"
)

const (
	// maxUpdatesBodyBytes caps the read mark POST body. A body is one id.
	maxUpdatesBodyBytes = 1024
	// muteNotFoundMessage is the 404 message of mute for a report that does not
	// exist or is not shown in any feed. Both read the same (T-03.2-06).
	muteNotFoundMessage = "This report is no longer available."
)

// UpdateWire is one row of the bell list or of the Updates part. It has no field
// for an account, a session, a voter cell, a coordinate, an email or a
// description, so a row cannot leak what the feed withholds (T-03.2-01).
type UpdateWire struct {
	// ID orders the list and is what the read mark sends back as up_to.
	ID int64 `json:"id" example:"41"`
	// Kind is the closed kind of the row.
	Kind string `json:"kind" enums:"reached_live,hidden,resolved,fading,reliable_nearby,more" example:"reached_live"`
	// ReportID is the report the row opens. It is absent for the rolled up row
	// and for a report that is no longer available, so a row never links to a
	// report the reader may not open.
	ReportID *int64 `json:"report_id,omitempty" example:"17"`
	// Available is false when the report was held, removed or has gone since the
	// update was made.
	Available bool `json:"available" example:"true"`
	// Unread is true until the reader marks the row read.
	Unread bool `json:"unread" example:"true"`
	// Event is line 1: the event words, built on the server.
	Event string `json:"event" example:"Now confirmed"`
	// ReportLine is the first line of the report's text (Q1 owner confirmed
	// 2026-10-18). It is absent for a finished Rescue report, an unavailable
	// report and the rolled up row. It is never the whole description.
	ReportLine string `json:"report_line,omitempty" example:"Water over the bridge"`
	// Category is the page's own category word, empty when unavailable.
	Category string `json:"category" example:"Flood"`
	// Severity is the severity word, empty when unavailable.
	Severity string `json:"severity" example:"Critical"`
	// SeverityLevel is the raw closed value for the severity dot colour.
	SeverityLevel string `json:"severity_level" enums:"low,medium,critical,''" example:"critical"`
	// Trust is the trust sentence for the report now, chosen when the list is read.
	Trust string `json:"trust" example:"Confirmed by 3 people at different spots."`
	// When is how long ago, in the app's short words.
	When string `json:"when" example:"12 min ago"`
	// Label is the row's accessible name, built whole on the server (UI-SPEC P32-C8).
	Label string `json:"label" example:"New. Now confirmed. Flood, Critical. Confirmed by 3 people at different spots. 12 minutes ago"`
	// CreatedAt is when the update was written.
	CreatedAt time.Time `json:"created_at"`
	// MoreCount is how many changes the rolled up row stands for. Only that row
	// carries it.
	MoreCount int `json:"more_count,omitempty" example:"4"`
}

// UpdatesResponse is the answer of GET /updates: the unread count capped at 99
// and the latest 10 rows, newest first.
type UpdatesResponse struct {
	Unread  int          `json:"unread" example:"3"`
	Updates []UpdateWire `json:"updates"`
}

// MarkReadRequest is the body of POST /updates/read: the id of the newest
// update the reader was shown. Only the caller's own rows up to it are marked.
type MarkReadRequest struct {
	UpTo int64 `json:"up_to" example:"41"`
}

// MutedReportWire is one row of the Muted reports window of Settings. An
// unavailable row carries report_id, available false and the mute time only:
// report_id is always present because Unmute needs it.
type MutedReportWire struct {
	ReportID      int64     `json:"report_id" example:"17"`
	Available     bool      `json:"available" example:"true"`
	Text          string    `json:"text" example:"Water over the bridge"`
	Category      string    `json:"category" example:"Flood"`
	Severity      string    `json:"severity" example:"Critical"`
	SeverityLevel string    `json:"severity_level" enums:"low,medium,critical,''" example:"critical"`
	MutedAt       time.Time `json:"muted_at"`
}

// MutedReportsResponse is the answer of GET /me/muted-reports.
type MutedReportsResponse struct {
	Reports []MutedReportWire `json:"reports"`
}

// relativeTimeLong is profileRelativeTime's rounding in whole words, for the
// accessible name of a row: "just now", "1 minute ago", "12 minutes ago",
// "1 hour ago", "3 hours ago", "1 day ago", "2 days ago" (UI-SPEC P32-C8).
func relativeTimeLong(t, now time.Time) string {
	diff := now.Sub(t)
	if diff < 0 {
		diff = 0
	}
	plural := func(n int, unit string) string {
		if n == 1 {
			return "1 " + unit + " ago"
		}
		return strconv.Itoa(n) + " " + unit + "s ago"
	}
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return plural(max(1, int(diff.Round(time.Minute)/time.Minute)), "minute")
	case diff < 24*time.Hour:
		return plural(max(1, int(diff.Round(time.Hour)/time.Hour)), "hour")
	default:
		return plural(max(1, int(diff.Round(24*time.Hour)/(24*time.Hour))), "day")
	}
}

// sentence ends s with a full stop unless it already ends a sentence, so an
// event line that carries its own final stop is not doubled.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	switch s[len(s)-1] {
	case '.', '!', '?':
		return s
	}
	return s + "."
}

// updateCategoryWord is the page's category word for a closed category, empty
// for the empty category of an unavailable row.
func updateCategoryWord(c service.Category) string {
	if c == "" {
		return ""
	}
	if w, ok := categoryLabels[c]; ok {
		return w
	}
	return categoryLabels[service.CategoryOther]
}

// updateSeverityWord is the severity word, empty for the empty severity of an
// unavailable row.
func updateSeverityWord(s service.Severity) string {
	if s == "" {
		return ""
	}
	return severityWord(s)
}

// updateSeverityLevel is the raw closed severity value for the dot colour, empty
// when there is none. Anything outside the closed set reads low, like the word.
func updateSeverityLevel(s service.Severity) string {
	switch s {
	case "":
		return ""
	case service.SeverityCritical, service.SeverityMedium:
		return string(s)
	default:
		return string(service.SeverityLow)
	}
}

// updateRowLabel is the accessible name of one row (UI-SPEC P32-C8): "New. "
// only when unread, line 1, the report line as its own sentence, "{Category},
// {Severity}.", the trust sentence and the time in full words. A part with no
// words is left out, so an unavailable row and the rolled up row read as their
// one line and their time.
func updateRowLabel(item service.UpdateItem, now time.Time) string {
	var parts []string
	if item.Unread {
		parts = append(parts, "New.")
	}
	if s := sentence(item.Event); s != "" {
		parts = append(parts, s)
	}
	if s := sentence(item.ReportLine); s != "" {
		parts = append(parts, s)
	}
	if item.Category != "" {
		parts = append(parts, updateCategoryWord(item.Category)+", "+updateSeverityWord(item.Severity)+".")
	}
	if s := sentence(item.Trust); s != "" {
		parts = append(parts, s)
	}
	parts = append(parts, relativeTimeLong(item.CreatedAt, now))
	return strings.Join(parts, " ")
}

// updateWire maps one service row to its wire shape. All words come from the
// service row or the page tables; nothing is composed from a request.
func updateWire(item service.UpdateItem, now time.Time) UpdateWire {
	return UpdateWire{
		ID:            item.ID,
		Kind:          string(item.Kind),
		ReportID:      item.ReportID,
		Available:     item.Available,
		Unread:        item.Unread,
		Event:         item.Event,
		ReportLine:    item.ReportLine,
		Category:      updateCategoryWord(item.Category),
		Severity:      updateSeverityWord(item.Severity),
		SeverityLevel: updateSeverityLevel(item.Severity),
		Trust:         item.Trust,
		When:          profileRelativeTime(item.CreatedAt, now),
		Label:         updateRowLabel(item, now),
		CreatedAt:     item.CreatedAt.UTC(),
		MoreCount:     item.MoreCount,
	}
}

// GetUpdates produces the handler of GET /updates.
//
// @Summary Read the bell: the unread count and the latest updates
// @Description Brings the signed in account's updates up to date and answers the unread count,
// @Description capped at 99, and the latest 10 updates, newest first. The poll itself is what
// @Description checks the reports you posted or voted on, so the bell polls this route. It takes
// @Description no parameter and no location. Every sentence is written by the server. A report
// @Description that was held, removed or has gone reads as an update that is no longer available,
// @Description with no detail. The answer is never cached.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags updates
// @Produce json
// @Success 200 {object} UpdatesResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /updates [get]
func GetUpdates(updates *service.UpdatesService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			// A route mounted outside the gated group, a programming error.
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// The poll is the producer (D-04). A failed refresh is logged and the
		// stored list is still worth showing.
		if err := updates.Refresh(r.Context(), acc.ID); err != nil {
			log.Printf("handlers: GetUpdates: refresh: %v", err)
		}
		page, err := updates.List(r.Context(), acc.ID, service.BellListSize)
		if err != nil {
			log.Printf("handlers: GetUpdates: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		now := time.Now()
		resp := UpdatesResponse{Unread: page.Unread, Updates: make([]UpdateWire, 0, len(page.Items))}
		for _, item := range page.Items {
			resp.Updates = append(resp.Updates, updateWire(item, now))
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
	}
}

// MarkUpdatesRead produces the handler of POST /updates/read.
//
// @Summary Mark your updates read up to an id
// @Description Marks the signed in account's own unread updates up to and including up_to as read.
// @Description Send the id of the newest update shown, so an update that arrived after the list
// @Description was drawn stays unread. It changes only your own rows. An up_to below 1 gets 400.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags updates
// @Accept json
// @Produce json
// @Param body body MarkReadRequest true "The id of the newest update shown"
// @Success 204 "Marked read"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /updates/read [post]
func MarkUpdatesRead(updates *service.UpdatesService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUpdatesBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var req MarkReadRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		if _, err := updates.MarkRead(r.Context(), acc.ID, req.UpTo); err != nil {
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: MarkUpdatesRead: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// MuteReport produces the handler of POST /reports/{id}/mute.
//
// @Summary Turn updates off for a report
// @Description Stops updates about one report for the signed in account only. Nobody else is told,
// @Description and the report itself does not change. Send no body or an empty object. A repeat
// @Description call changes nothing. A report that does not exist, or that is not shown in any
// @Description feed, gets 404.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags reports
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Success 204 "Updates are off for this report"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/mute [post]
func MuteReport(mutes *service.MuteService) http.HandlerFunc {
	return muteHandler("MuteReport", mutes.Mute)
}

// UnmuteReport produces the handler of POST /reports/{id}/unmute.
//
// @Summary Turn updates back on for a report
// @Description Undoes a mute for the signed in account only. Send no body or an empty object. A
// @Description repeat call changes nothing. It works for a report that has since been held, removed
// @Description or gone, so a mute can always be cleared.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags reports
// @Accept json
// @Produce json
// @Param id path int true "Report id"
// @Success 204 "Updates are on again for this report"
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports/{id}/unmute [post]
func UnmuteReport(mutes *service.MuteService) http.HandlerFunc {
	return muteHandler("UnmuteReport", mutes.Unmute)
}

// muteHandler is the one body of mute and unmute: the account from the gate, the
// report id from the path, an empty body, one service call and a 204.
func muteHandler(name string, act func(ctx context.Context, accountID, reportID int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		reportID, ok := parseReportID(w, r)
		if !ok {
			return
		}
		if !decodeEmptyBody(w, r) {
			return
		}
		if err := act(r.Context(), acc.ID, reportID); err != nil {
			if errors.Is(err, service.ErrReportNotFound) {
				writeFieldError(w, http.StatusNotFound, "id", muteNotFoundMessage)
				return
			}
			log.Printf("handlers: %s: %v", name, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// GetMutedReports produces the handler of GET /me/muted-reports.
//
// @Summary List the reports you muted
// @Description Lists the signed in account's muted reports, newest mute first, for the Muted reports
// @Description window of Settings. A report that is held, removed or has gone shows available false
// @Description with no text, category or severity, and still carries report_id so the mute can be
// @Description undone. A finished Rescue report shows no text. The answer is never cached.
// @Description Requires a session verified by email; an unverified caller receives 401.
// @Tags me
// @Produce json
// @Success 200 {object} MutedReportsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /me/muted-reports [get]
func GetMutedReports(mutes *service.MuteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		rows, err := mutes.MutedReports(r.Context(), acc.ID)
		if err != nil {
			log.Printf("handlers: GetMutedReports: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		resp := MutedReportsResponse{Reports: make([]MutedReportWire, 0, len(rows))}
		for _, row := range rows {
			wire := MutedReportWire{ReportID: row.ReportID, Available: row.Available, MutedAt: row.MutedAt.UTC()}
			if row.Available {
				wire.Text = row.Text
				wire.Category = updateCategoryWord(service.Category(row.Category))
				wire.Severity = updateSeverityWord(service.Severity(row.Severity))
				wire.SeverityLevel = updateSeverityLevel(service.Severity(row.Severity))
			}
			resp.Reports = append(resp.Reports, wire)
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, resp)
	}
}
