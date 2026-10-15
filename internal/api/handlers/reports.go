// Package handlers implements the HTTP handlers for Pinalert's JSON API.
// Handlers decode/encode only — all validation, expiry computation, and
// geo-math live in internal/service; all SQL lives behind the sqlc-generated
// Querier. This file also declares the public JSON report shape explicitly,
// field by field, so a column added to the store later cannot silently
// start appearing in a response (see 01-RESEARCH.md's Security Domain:
// report read paths must never return session_id).
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"

	"pinalert/internal/account"
	"pinalert/internal/service"
	"pinalert/internal/session"
)

// maxSubmitBodyBytes caps the POST body so an unbounded request can't be
// used as a cheap denial-of-service vector.
const maxSubmitBodyBytes = 64 * 1024

// Nearby-query radius defaults and bounds (FOUND-03): a single request must
// never be able to demand an arbitrarily wide Haversine evaluation.
const (
	defaultRadiusKm = 10.0
	minRadiusKm     = 0.1
	maxRadiusKm     = 50.0
)

// SubmitReportRequest is the raw JSON shape a client posts to POST
// /api/reports. Every field that the server itself computes (id, geohash,
// created_at, expires_at) is deliberately absent here — decode with
// DisallowUnknownFields so an attempt to set one of them is a loud 400, not
// a silently dropped field. Exported and hand-declared field by field (never
// aliasing a store row) so swag can document it by name and so a column
// added to the store later cannot silently change this contract.
type SubmitReportRequest struct {
	// Category is one of the nine canonical report categories (internal/service.Categories).
	Category string `json:"category" enums:"flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other" example:"flood"`
	// Severity affects triage ordering only; self-declaring critical never bypasses the provisional-visibility gate (internal/service.Severities).
	Severity string `json:"severity" enums:"low,medium,critical" example:"medium"`
	// Description is a free text account of the report, 10 to 1000 characters after trimming, counted as characters rather than bytes.
	Description string `json:"description" example:"Water rising fast near the market bridge."`
	// Latitude must be between -90 and 90.
	Latitude float64 `json:"latitude" example:"13.0827"`
	// Longitude must be between -180 and 180.
	Longitude float64 `json:"longitude" example:"80.2707"`
	// ShelterCapacityStatus is required when category is shelter_open and forbidden for every other category (internal/service.CapacityStatuses).
	ShelterCapacityStatus *string `json:"shelter_capacity_status,omitempty" enums:"available,limited,full,closed" example:"available"`
	// ShelterHeadcount is optional and only valid alongside category shelter_open.
	ShelterHeadcount *int `json:"shelter_headcount,omitempty" example:"42"`
}

// ReportResponse is the public JSON shape of a report returned by POST
// /api/reports (the GET /api/reports element shape is FeedReportResponse
// below). Declared field by field rather than reusing any store row type —
// see package doc comment. It never includes session_id (threat register
// T-01-02).
type ReportResponse struct {
	ID          int64   `json:"id" example:"42"`
	Category    string  `json:"category" enums:"flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other" example:"flood"`
	Severity    string  `json:"severity" enums:"low,medium,critical" example:"critical"`
	Description string  `json:"description" example:"Water rising fast near the market bridge."`
	Latitude    float64 `json:"latitude" example:"13.0827"`
	Longitude   float64 `json:"longitude" example:"80.2707"`
	Geohash     string  `json:"geohash" example:"tdr1qgzp"`
	// ShelterCapacityStatus is present only for shelter_open reports.
	ShelterCapacityStatus *string `json:"shelter_capacity_status,omitempty" enums:"available,limited,full,closed" example:"available"`
	ShelterHeadcount      *int    `json:"shelter_headcount,omitempty" example:"42"`
	CreatedAt             string  `json:"created_at" example:"2026-09-05T14:03:00.000Z"`
	ExpiresAt             string  `json:"expires_at" example:"2026-09-06T14:03:00.000Z"`
	// DistanceKm is present only in GET /api/reports responses (nil on the report just submitted).
	DistanceKm *float64 `json:"distance_km,omitempty" example:"1.42"`
	// Held is true only when the text check could not finish and the report is not
	// serious, so it is stored but shown in no feed or map until a background check
	// finishes. A published report, borderline text included, carries false. Never omitted.
	Held bool `json:"held" example:"false"`
}

// SubmitReportResponse wraps the single report created by a successful
// POST /api/reports.
type SubmitReportResponse struct {
	Report ReportResponse `json:"report"`
}

// FeedReportResponse is the public JSON shape of a report returned by GET
// /api/reports: the twelve fields ReportResponse also carries, plus the
// read-only fields the resolver and the viewer's own standing contribute (the
// visibility, the view, the history of a finished report and the viewer's own
// vote, resolution and take backs), plus the trust object (D-21).
// Hand-declared field by field, never embedding ReportResponse and never
// aliasing a store row, exactly as this file's package doc comment
// requires: each public JSON shape is declared explicitly so a change to
// one can never silently alter the other. The duplication with
// ReportResponse is deliberate for that same reason.
type FeedReportResponse struct {
	ID          int64   `json:"id" example:"42"`
	Category    string  `json:"category" enums:"flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other" example:"flood"`
	Severity    string  `json:"severity" enums:"low,medium,critical" example:"critical"`
	Description string  `json:"description" example:"Water rising fast near the market bridge."`
	Latitude    float64 `json:"latitude" example:"13.0827"`
	Longitude   float64 `json:"longitude" example:"80.2707"`
	Geohash     string  `json:"geohash" example:"tdr1qgzp"`
	// ShelterCapacityStatus is present only for shelter_open reports.
	ShelterCapacityStatus *string  `json:"shelter_capacity_status,omitempty" enums:"available,limited,full,closed" example:"available"`
	ShelterHeadcount      *int     `json:"shelter_headcount,omitempty" example:"42"`
	CreatedAt             string   `json:"created_at" example:"2026-09-05T14:03:00.000Z"`
	ExpiresAt             string   `json:"expires_at" example:"2026-09-06T14:03:00.000Z"`
	DistanceKm            *float64 `json:"distance_km,omitempty" example:"1.42"`
	// Visibility is the resolver's own answer, serialised verbatim from
	// service.ReportView.Visibility; the client renders it and never
	// recomputes it (TRUST-02, T-02-04). "retracted" is one of the four
	// values of the shared type. It never appears in the active or the
	// disputed view (D-12) and appears only in the past view, where the
	// report is listed as resolved (D-25).
	Visibility string `json:"visibility" enums:"hidden,provisional,live,retracted" example:"live"`
	// VisibilityReason is the resolver's own explanation for Visibility
	// above, from the SAME closed slug set votes.go's CastVoteResponse.Reason
	// declares, so 02-06's display-copy mapping has one contract to key off
	// regardless of which endpoint delivered it.
	VisibilityReason string `json:"visibility_reason" enums:"resolved,critical_bypasses_gates,disputed,awaiting_second_independent_confirmation,confirmed" example:"confirmed"`
	// YourVote is the calling account's own current content vote on this
	// report — confirm, dispute, or null. Deliberately without omitempty,
	// so the key is always present and explicitly null when the caller has
	// no standing content vote. It is viewer-specific, which makes this
	// response uncacheable across callers.
	YourVote *string `json:"your_vote" enums:"confirm,dispute" example:"confirm"`
	// IsOwnReport is true when the calling account submitted this report.
	// 02-05 omits the confirm/dispute controls when true (D-03) rather than
	// rendering a button that will 403 — a convenience for the client,
	// never the control: service.VotingService.CastVote refuses the
	// self-vote server-side and 02-03b returns 403 for it regardless of
	// what the client renders. Derived from a reporter account id the
	// server looks up and never serialises: this response carries a
	// boolean, never an account id or an email (T-01-02).
	IsOwnReport bool `json:"is_own_report" example:"false"`
	// Trust is the server built trust object the vote response also carries
	// for this report (D-21). It is always present, the same for every
	// viewer, and never contains an identifier.
	Trust TrustResponse `json:"trust"`
	// View is the one tab this report belongs to right now, from the
	// resolver's own answer at the read's clock: active, disputed, resolved
	// (closed by a resolve) or past (ran out its time). The client renders
	// it and never recomputes it.
	View string `json:"view" enums:"active,disputed,resolved,past" example:"active"`
	// History says how a finished report ended. It is null for an active or
	// a disputed report and the key is always present.
	History *HistoryResponse `json:"history"`
	// Approximate is true when the position and the geohash are a blurred
	// point and the description is empty, which is how a finished Rescue
	// report leaves the API (D-23).
	Approximate bool `json:"approximate" example:"false"`
	// YourResolution is the calling account's own standing resolution vote on
	// this report, or null. The key is always present.
	YourResolution *string `json:"your_resolution" enums:"resolve,reopen,unresolve" example:"resolve"`
	// UndosLeft is how many take backs the calling account has left on this
	// report, from 0 to 2.
	UndosLeft int `json:"undos_left" minimum:"0" maximum:"2" example:"2"`
}

// HistoryResponse is how a finished report ended. State is resolved or
// expired. The other keys are present only where they apply: closed_by and
// closed_text for a resolved report, outcome, outcome_label and lifetime_text
// for one that expired. The server builds every text; the client only shows it.
type HistoryResponse struct {
	State        string `json:"state" enums:"resolved,expired" example:"resolved"`
	At           string `json:"at" example:"2026-09-05T14:03:00.000Z"`
	ClosedBy     string `json:"closed_by,omitempty" enums:"you,poster,people,poster_and_people,you_and_people" example:"poster"`
	ClosedText   string `json:"closed_text,omitempty" example:"Resolved by the poster"`
	Outcome      string `json:"outcome,omitempty" enums:"corroborated,contradicted,unconfirmed,neutral" example:"corroborated"`
	OutcomeLabel string `json:"outcome_label,omitempty" example:"Confirmed by 3 people nearby"`
	LifetimeText string `json:"lifetime_text,omitempty" example:"after 6 hrs"`
}

// RecentResponse is what happened lately when the active tab is empty: how
// many reports near the reader were resolved and how many faded in the last 24
// hours.
type RecentResponse struct {
	Resolved24h int `json:"resolved_24h" example:"2"`
	Expired24h  int `json:"expired_24h" example:"1"`
}

// CutResponse says how complete each group of the past view is. Resolved and
// Expired are true when that group had more than 25 entries kept and the list
// shows the newest 25. ResolvedPartial and ExpiredPartial are separate: true when
// the area had more reports than the server could read for that group, so some
// are not shown even when fewer than 25 are. The open read feeds the resolved
// group only; the expired read feeds both groups.
type CutResponse struct {
	Resolved        bool `json:"resolved" example:"false"`
	Expired         bool `json:"expired" example:"false"`
	ResolvedPartial bool `json:"resolved_partial" example:"false"`
	ExpiredPartial  bool `json:"expired_partial" example:"false"`
}

// FocusResponse answers a report link. Status is ok (report is the report),
// too_old (it ended more than seven days ago) or unavailable (no such report,
// or one no tab lists). Report is null unless status is ok. Not found and every
// unlisted state share the one unavailable word so the answer never reveals
// why.
type FocusResponse struct {
	Status string              `json:"status" enums:"ok,too_old,unavailable" example:"ok"`
	Report *FeedReportResponse `json:"report"`
}

// ReportListResponse wraps what GET /api/reports returns for one view:
// the tab echoed back, the reports of that tab and the optional recent, cut
// and focus objects.
type ReportListResponse struct {
	// View is the tab these reports belong to. resolved is only an alias of
	// past on the way in and is answered as past.
	View string `json:"view" enums:"active,disputed,past" example:"active"`
	// Reports is never null. The past view lists the resolved group first and
	// then the expired group, each newest first.
	Reports []FeedReportResponse `json:"reports"`
	// Recent is present only on an active, unfiltered view with no reports.
	Recent *RecentResponse `json:"recent,omitempty"`
	// Cut is present only on the past view.
	Cut *CutResponse `json:"cut,omitempty"`
	// Focus is present only when the report parameter was sent.
	Focus *FocusResponse `json:"focus,omitempty"`
}

// reportToResponse maps the report SubmitChecked stored, and whether it is
// held, onto the wire shape of POST /api/reports. DistanceKm stays nil.
func reportToResponse(res service.SubmitResult) ReportResponse {
	r := res.Report
	var capacityStatus *string
	if r.ShelterCapacityStatus != nil {
		v := string(*r.ShelterCapacityStatus)
		capacityStatus = &v
	}
	return ReportResponse{
		ID:                    r.ID,
		Category:              string(r.Category),
		Severity:              string(r.Severity),
		Description:           r.Description,
		Latitude:              r.Latitude,
		Longitude:             r.Longitude,
		Geohash:               r.Geohash,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      r.ShelterHeadcount,
		CreatedAt:             r.CreatedAt.Format(rfc3339Milli),
		ExpiresAt:             r.ExpiresAt.Format(rfc3339Milli),
		Held:                  res.Held,
	}
}

// reportViewToResponse maps a service.ReportView (a report plus the
// resolver's answer and the viewer's own standing) onto the wire shape
// GET /api/reports returns. YourVote is nil exactly when v.ViewerVote is
// the empty service.VoteValue (no standing content vote); otherwise it
// points at the vote's string value. YourResolution follows the same rule
// for v.ViewerResolution. Deliberately not deduplicated with
// reportToResponse, the same reason FeedReportResponse and ReportResponse
// are declared as two separate structs above.
//
// This is the one function that builds a FeedReportResponse, so the approximate
// guard below covers every path that puts a report on the wire (D-23). The
// service already blanks the description of a blurred report; this is the
// second, independent line of defence.
func reportViewToResponse(v service.ReportView) FeedReportResponse {
	var capacityStatus *string
	if v.ShelterCapacityStatus != nil {
		s := string(*v.ShelterCapacityStatus)
		capacityStatus = &s
	}
	var yourVote *string
	if v.ViewerVote != "" {
		s := string(v.ViewerVote)
		yourVote = &s
	}
	var yourResolution *string
	if v.ViewerResolution != "" {
		s := string(v.ViewerResolution)
		yourResolution = &s
	}
	description := v.Description
	if v.Approximate && description != "" {
		log.Printf("handlers: reportViewToResponse: report %d is approximate but still carried a description; blanked", v.ID)
		description = ""
	}
	return FeedReportResponse{
		ID:                    v.ID,
		Category:              string(v.Category),
		Severity:              string(v.Severity),
		Description:           description,
		Latitude:              v.Latitude,
		Longitude:             v.Longitude,
		Geohash:               v.Geohash,
		ShelterCapacityStatus: capacityStatus,
		ShelterHeadcount:      v.ShelterHeadcount,
		CreatedAt:             v.CreatedAt.Format(rfc3339Milli),
		ExpiresAt:             v.ExpiresAt.Format(rfc3339Milli),
		DistanceKm:            v.DistanceKm,
		Visibility:            string(v.Visibility),
		VisibilityReason:      string(v.VisibilityReason),
		YourVote:              yourVote,
		IsOwnReport:           v.IsOwnReport,
		Trust:                 trustToResponse(v.Trust),
		View:                  string(v.View),
		History:               historyToResponse(v.History),
		Approximate:           v.Approximate,
		YourResolution:        yourResolution,
		UndosLeft:             v.UndosLeft,
	}
}

// historyToResponse maps the service's history facts onto the wire object, or
// nil for a report that has none. The time is written in UTC with
// milliseconds, like every other time on the wire.
func historyToResponse(h *service.HistoryFacts) *HistoryResponse {
	if h == nil {
		return nil
	}
	return &HistoryResponse{
		State:        h.State,
		At:           h.At.UTC().Format(rfc3339Milli),
		ClosedBy:     string(h.ClosedBy),
		ClosedText:   h.ClosedText,
		Outcome:      string(h.Outcome),
		OutcomeLabel: h.OutcomeLabel,
		LifetimeText: h.LifetimeText,
	}
}

const rfc3339Milli = "2006-01-02T15:04:05.000Z07:00"

// ErrorDetail names the single input field that failed validation (or
// "body" for a malformed request) and a user-safe message.
type ErrorDetail struct {
	Field   string `json:"field" example:"category"`
	Message string `json:"message" example:"Choose a category to continue."`
}

// ErrorResponse is the JSON shape of every 400 and 500 response this API
// returns. Exported so swag can document it by name.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// SubmitReport handles POST /api/reports: decode, validate, screen the
// description and persist under the caller's session id. Reaching this handler
// at all requires a session verified by email through the magic link flow, and
// the access gate in internal/api/gate.go (01.1-04) refuses an unverified
// caller with 401 before this function ever runs. A ValidationError maps to
// 400 with the offending field named. A description the text check refuses maps
// to 422 on the field description with the one exported message for the reason,
// and nothing is stored. Any other error maps to a generic 500 with the detail
// logged server side only, so database errors never reach a client. A 201 body
// carries held, true when the text check could not finish and the report is
// not serious (D-02).
//
// @Summary Submit a new emergency report
// @Description Creates a location-tagged report under the caller's session. Requires a session
// @Description verified by email through the magic-link flow (POST /auth/request-link, then
// @Description GET /auth/verify) — an unverified caller receives 401. id, geohash, created_at
// @Description and expires_at are computed server-side and can never be set by the client.
// @Tags reports
// @Accept json
// @Produce json
// @Param body body SubmitReportRequest true "Report to submit"
// @Success 201 {object} SubmitReportResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /reports [post]
func SubmitReport(svc *service.ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := session.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		// The gate already resolved a verified account, so a missing one is a
		// programming error, the same shape NearbyReports uses.
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req SubmitReportRequest
		if err := dec.Decode(&req); err != nil {
			writeFieldError(w, http.StatusBadRequest, "body", "Request body is missing or malformed.")
			return
		}

		var capacityStatus *service.CapacityStatus
		if req.ShelterCapacityStatus != nil {
			cs := service.CapacityStatus(*req.ShelterCapacityStatus)
			capacityStatus = &cs
		}

		in := service.SubmitInput{
			Category:              service.Category(req.Category),
			Severity:              service.Severity(req.Severity),
			Description:           req.Description,
			Latitude:              req.Latitude,
			Longitude:             req.Longitude,
			ShelterCapacityStatus: capacityStatus,
			ShelterHeadcount:      req.ShelterHeadcount,
		}

		if err := service.ValidateSubmitInput(in); err != nil {
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: SubmitReport: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		res, err := svc.SubmitChecked(r.Context(), sessionID, acc.ID, in)
		if err != nil {
			var tr service.TextRejectedError
			if errors.As(err, &tr) {
				writeFieldError(w, http.StatusUnprocessableEntity, "description", service.RejectMessage(tr.Reason))
				return
			}
			var ve service.ValidationError
			if errors.As(err, &ve) {
				writeFieldError(w, http.StatusBadRequest, ve.Field, ve.Message)
				return
			}
			log.Printf("handlers: SubmitReport: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, SubmitReportResponse{Report: reportToResponse(res)})
	}
}

// NearbyReports handles GET /api/reports: parse lat/lon/radius_km/view/
// category/report (and the older show_disputed) from the query string, run the
// indexed bbox+Haversine reads plus the resolver over them via svc, and return
// the reports of one view nearest-first (the past view newest-first). This is
// the one endpoint that serves the map, the list, the Past tab, the counts for
// an empty Active tab and a report link (01-RESEARCH.md Pattern 3, D-05): there
// is no separate route per tab. Reaching this handler also requires a verified
// session (see SubmitReport's doc comment); the gate refuses an unverified
// reader with 401 before any report data is looked up.
//
// @Summary List the reports near a point for one view: active, disputed or past
// @Description Runs the indexed bounding-box prefilter followed by exact Haversine distance,
// @Description then resolves each report fresh via the shared resolver. This single endpoint serves
// @Description the map, the list, the Past tab, the counts for an empty Active tab and a report link;
// @Description there is no separate route per tab.
// @Description The view parameter picks the tab. active (the default) is Live and Provisional reports,
// @Description nearest first. disputed is only reports currently hidden by disputes, replacing the
// @Description default view rather than adding to it; a critical or rescue-needed report stays Live
// @Description however heavily it is disputed, so it appears only in the active view. past is what was
// @Description resolved and what ran out its time in the last seven days, the resolved group first and
// @Description then the expired group, each newest first and cut at 25 with the cut object saying so.
// @Description resolved is accepted as a read only alias of past and answered as past. A resolved
// @Description (retracted) report is never returned by the active or the disputed view; it is listed
// @Description only in the past view, and no past entry changes an active count.
// @Description show_disputed=true still selects the disputed view so old links keep working; sent with
// @Description another view it is refused with 400 on the field view.
// @Description The category parameter keeps one report category in the chosen view and is refused with
// @Description 400 when it is not one of the nine.
// @Description The report parameter is a report id from a link. When it is sent the focus object answers
// @Description it: ok with that report even when it lies outside the radius and outside the seven day
// @Description window, too_old when it ended more than seven days ago, or unavailable when there is no such
// @Description report or no tab lists it. Not found and every unlisted state share the one unavailable
// @Description answer. The reports array still holds the reader's own area.
// @Description recent ({resolved_24h, expired_24h}) is present only when the view is active, no category
// @Description was sent and there are no reports, so an empty tab can say what happened lately.
// @Description A finished Rescue report never leaves the API exact on the past view or on a link: its
// @Description description is empty, its latitude, longitude and geohash are a blurred point,
// @Description approximate is true and distance_km is measured to that point. A live Rescue report and
// @Description finished Flood, Shelter and Power reports stay exact.
// @Description Every report carries your_vote, your_resolution, undos_left and is_own_report relative to
// @Description the calling verified account. Requires a session verified by email through the
// @Description magic-link flow; an unverified caller receives 401 and no report data.
// @Description Every report also carries a trust object: how many people at different nearby spots confirmed and disputed it, counted once per spot and not once per vote.
// @Description lat, lon and radius_km must be finite numbers; NaN is refused with 400 and the name of the parameter.
// @Tags reports
// @Produce json
// @Param lat query number true "Latitude of the query center (required, -90 to 90)"
// @Param lon query number true "Longitude of the query center (required, -180 to 180)"
// @Param radius_km query number false "Search radius in kilometers (defaults to 10, bounded to 0.1-50)" minimum(0.1) maximum(50) default(10)
// @Param view query string false "Which tab to list: active (the default), disputed or past. resolved is an alias of past." Enums(active,disputed,past,resolved)
// @Param show_disputed query boolean false "Older way to select the disputed view. true selects disputed; with another view it is refused (defaults to false)"
// @Param category query string false "Keep only this report category in the chosen view (default: all)" Enums(flood,earthquake,fire,storm_cyclone,road_blocked,power_outage,shelter_open,rescue_needed,other)
// @Param report query integer false "A report id from a link; answered in the focus object" minimum(1)
// @Success 200 {object} ReportListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Router /reports [get]
func NearbyReports(svc *service.ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Reaching this handler at all means the gate already resolved a
		// verified account, so a missing one is a programming error (a
		// route mounted outside the gated group), not a client condition,
		// the same shape SubmitReport uses for a missing session id.
		acc, ok := account.FromContext(r.Context())
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// UAT gap 3 (02-UAT.md, closed by 02-09): this feed response is a
		// personalised, freshness-critical view. It carries your_vote,
		// is_own_report, and a visibility recomputed fresh on every read
		// (service.Resolve), so an uncached-but-cacheable JSON response
		// is eligible for a browser's own heuristic caching, and a fresh
		// fetch() could still be answered from a pre-vote snapshot even on
		// a page that DID re-run its JavaScript after a Back navigation.
		// Set before any query-parameter parsing below, so every exit path
		// from this handler (the 200, every 400 from the validators that
		// follow, and the 500 above) carries it. Cross-reference page.go's
		// DEC-Q header, which applies the same reasoning to the document
		// itself.
		//
		// Deliberately NOT set inside writeJSON: that helper is also called
		// by auth.go's link-request route, this file's own submit route
		// and error helper, and votes.go's cast-vote route, six call
		// sites across three files nobody examined for this change.
		// Blanket-applying a cache directive there would silently change
		// the response contract of five endpoints whose only stated
		// purpose here is a feed-freshness fix.
		w.Header().Set("Cache-Control", "no-store")

		q := r.URL.Query()

		lat, ok := parseCoordinate(w, q, "lat", -90, 90)
		if !ok {
			return
		}
		lon, ok := parseCoordinate(w, q, "lon", -180, 180)
		if !ok {
			return
		}

		radiusKm := defaultRadiusKm
		if raw := q.Get("radius_km"); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			// ParseFloat accepts the unsigned words NaN and nan, and NaN
			// passes every < and > range check below, so refuse it here.
			if err != nil || math.IsNaN(parsed) {
				writeFieldError(w, http.StatusBadRequest, "radius_km", "radius_km must be a number.")
				return
			}
			radiusKm = parsed
		}
		if radiusKm < minRadiusKm || radiusKm > maxRadiusKm {
			writeFieldError(w, http.StatusBadRequest, "radius_km",
				"radius_km must be between 0.1 and 50.")
			return
		}

		// The default view is deliberately closed: omitting a parameter must
		// never widen the result, and an unparseable value is rejected rather
		// than silently treated as the default, because silently ignoring a
		// filter the caller asked for is how a user ends up trusting a view
		// that is not the view they requested (D-04, D-17, D-28). All three
		// parameters are checked before the service is reached.
		view, problem := parseFeedView(q)
		if problem != nil {
			writeFieldError(w, http.StatusBadRequest, problem.Field, problem.Message)
			return
		}
		category, problem := parseCategory(q)
		if problem != nil {
			writeFieldError(w, http.StatusBadRequest, problem.Field, problem.Message)
			return
		}
		reportID, hasReport, problem := parseReportParam(q)
		if problem != nil {
			writeFieldError(w, http.StatusBadRequest, problem.Field, problem.Message)
			return
		}

		// ViewerAccountID comes from the gate's context, never from the
		// query string: a client cannot ask what some other account
		// voted, resolved or took back.
		resp := ReportListResponse{View: string(view)}
		var views []service.ReportView
		switch view {
		case service.FeedViewPast:
			page, err := svc.History(r.Context(), service.HistoryQuery{
				Latitude:        lat,
				Longitude:       lon,
				RadiusKm:        radiusKm,
				ViewerAccountID: acc.ID,
				Category:        category,
			})
			if err != nil {
				log.Printf("handlers: NearbyReports: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			views = page.Reports
			resp.Cut = &CutResponse{
				Resolved:        page.ResolvedCut,
				Expired:         page.ExpiredCut,
				ResolvedPartial: page.ResolvedPartial,
				ExpiredPartial:  page.ExpiredPartial,
			}
		default:
			nearby, err := svc.Nearby(r.Context(), service.NearbyQuery{
				Latitude:        lat,
				Longitude:       lon,
				RadiusKm:        radiusKm,
				ViewerAccountID: acc.ID,
				IncludeDisputed: view == service.FeedViewDisputed,
				Category:        category,
			})
			if err != nil {
				log.Printf("handlers: NearbyReports: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			views = nearby
		}

		// The counts for an empty Active tab (D-12): only the unfiltered
		// active view, and only when it came back empty, so a page with
		// reports and every filtered page cost nothing extra.
		if view == service.FeedViewActive && category == "" && len(views) == 0 {
			counts, err := svc.RecentHistoryCounts(r.Context(), service.RecentQuery{
				Latitude:  lat,
				Longitude: lon,
				RadiusKm:  radiusKm,
			})
			if err != nil {
				log.Printf("handlers: NearbyReports: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			resp.Recent = &RecentResponse{Resolved24h: counts.Resolved, Expired24h: counts.Expired}
		}

		// The report link (D-11, D-28): one more read with the same centre
		// and the same gate account.
		if hasReport {
			res, err := svc.Focus(r.Context(), service.FocusQuery{
				ReportID:        reportID,
				Latitude:        lat,
				Longitude:       lon,
				ViewerAccountID: acc.ID,
			})
			if err != nil {
				log.Printf("handlers: NearbyReports: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			focus := &FocusResponse{Status: string(res.Status)}
			if res.Status == service.FocusOK && res.View != nil {
				mapped := reportViewToResponse(*res.View)
				focus.Report = &mapped
			}
			resp.Focus = focus
		}

		responses := make([]FeedReportResponse, 0, len(views))
		for _, v := range views {
			responses = append(responses, reportViewToResponse(v))
		}
		resp.Reports = responses
		writeJSON(w, http.StatusOK, resp)
	}
}

// paramProblem names the one query parameter that failed validation and the
// message the 400 carries.
type paramProblem struct {
	Field   string
	Message string
}

// parseFeedView reads the view and the older show_disputed parameter and
// returns the one view they select. Pure and strict: matching is case
// sensitive, a blank value is the same as absent, resolved is a read only alias
// of past, and show_disputed=true is the disputed view and disagrees with any
// other explicit view. show_disputed is read first so a non boolean value keeps
// naming its own field (D-04, D-05, D-28).
func parseFeedView(q url.Values) (service.FeedView, *paramProblem) {
	showDisputed := false
	if raw := q.Get("show_disputed"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return "", &paramProblem{Field: "show_disputed", Message: "show_disputed must be true or false."}
		}
		showDisputed = parsed
	}

	view := service.FeedViewActive
	explicit := false
	switch raw := q.Get("view"); raw {
	case "":
		// Absent or blank: the default view.
	case "active":
		explicit = true
	case "disputed":
		view = service.FeedViewDisputed
		explicit = true
	case "past", "resolved":
		view = service.FeedViewPast
		explicit = true
	default:
		return "", &paramProblem{Field: "view", Message: "view must be active, disputed or past."}
	}

	if showDisputed {
		if explicit && view != service.FeedViewDisputed {
			return "", &paramProblem{Field: "view", Message: "view and show_disputed disagree."}
		}
		view = service.FeedViewDisputed
	}
	return view, nil
}

// parseCategory reads the optional category filter: absent or blank means all,
// anything else must be exactly one of service.Categories.
func parseCategory(q url.Values) (service.Category, *paramProblem) {
	raw := q.Get("category")
	if raw == "" {
		return "", nil
	}
	c := service.Category(raw)
	if !c.Valid() {
		return "", &paramProblem{Field: "category", Message: "category must be one of the report categories."}
	}
	return c, nil
}

// parseReportParam reads the optional report id of a link. It returns the id
// and whether one was sent. Only plain decimal digits that fit an int64 and
// are not zero are a report id.
func parseReportParam(q url.Values) (id int64, sent bool, problem *paramProblem) {
	raw := q.Get("report")
	if raw == "" {
		return 0, false, nil
	}
	bad := &paramProblem{Field: "report", Message: "report must be a report id."}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, false, bad
		}
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed < 1 {
		return 0, false, bad
	}
	return parsed, true, nil
}

// parseCoordinate reads and range-checks a required numeric query
// parameter, writing the 400 response itself and returning ok=false on any
// failure so the caller can just early-return.
func parseCoordinate(w http.ResponseWriter, q map[string][]string, name string, min, max float64) (value float64, ok bool) {
	raw := ""
	if vals, present := q[name]; present && len(vals) > 0 {
		raw = vals[0]
	}
	if raw == "" {
		writeFieldError(w, http.StatusBadRequest, name, name+" is required.")
		return 0, false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) {
		writeFieldError(w, http.StatusBadRequest, name, name+" must be a number.")
		return 0, false
	}
	if parsed < min || parsed > max {
		writeFieldError(w, http.StatusBadRequest, name, name+" is out of range.")
		return 0, false
	}
	return parsed, true
}

func writeFieldError(w http.ResponseWriter, status int, field, message string) {
	writeJSON(w, status, ErrorResponse{Error: ErrorDetail{Field: field, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("handlers: writing JSON response: %v", err)
	}
}
