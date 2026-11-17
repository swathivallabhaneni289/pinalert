package handlers

// demo_activity.go is the Demo's My activity place (plan 05-19, D-05, UI-SPEC
// 4c). It shows what the visitor did in this visit and nothing else: the
// reports confirmed and the reports disputed, plus Ended, which the page fills
// from the visit's own reports route (amended 2026-11-17, quick 261117-vuf: the
// practice reports posted left with the real page's Posted). The
// parts, the cards and the empty states are the real page's: the cards are
// built by the same buildActivityPage and newProfileReport that the signed in
// page uses, from the same service shapes, so a card reads exactly as it would
// for a real account.
//
// What the Demo leaves out, because it has no account, no bell and no queue:
// the Updates part, the reporter record card, the Waiting to send strip and the
// moderation notes. Nothing is stored: the handler reads no session and no
// account, writes no row and sets no cookie, and the cards are built in memory
// from the world, so reloading the page (which starts a fresh world) empties it.

import (
	"context"
	"html/template"
	"log"
	"net/http"

	"pinalert/internal/demo"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// demoActivityTemplate is the template DemoActivityPage executes.
const demoActivityTemplate = "demo_activity.html.tmpl"

// demoActivityPath is the Demo's My activity address, without a query.
var demoActivityPath = demoNavHrefs[NavActivity]

// demoActivityViewModel is exactly what demo_activity.html.tmpl reads: the
// shared Demo page data, the parts in order and the key of the part the address
// asked for (empty for the list).
type demoActivityViewModel struct {
	demoViewModel
	Parts       []activityPart
	CurrentPart string
}

// DemoActivityPage handles GET /demo/activity: the visit's My activity. The
// visit is the one named by ?v= when it is valid and alive, else a new world,
// as for the Demo's other places. ?part= picks one of the three parts on a phone
// (the list otherwise); updates and posted are not Demo parts and read as the
// list, like any other word. It is reached through DemoRoutes, behind the per IP visit
// limit.
func DemoActivityPage(tmpl *template.Template, store *demo.Store, cfg PageConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vm, world, ok := newDemoPlaceModel(w, r, store, cfg, NavActivity)
		if !ok {
			return
		}

		page, err := demoActivityData(r.Context(), world)
		if err != nil {
			log.Printf("handlers: DemoActivityPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		part := parseActivityPart(r.URL.Query().Get("part"), "")
		if part == activityPartUpdates {
			part = ""
		}
		view, err := buildActivityPage(page, part, world.Now())
		if err != nil {
			log.Printf("handlers: DemoActivityPage: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		from := demoFromWord(r)
		listHref := demoPlaceHref(demoActivityPath, vm.Visit, from)
		// The same three parts as the real list (quick 261117-vuf): Confirmed,
		// Disputed and Ended, with no Updates.
		shown := activityShownParts(view.Parts, view.CurrentPart)
		parts := make([]activityPart, 0, len(shown))
		for _, p := range shown {
			// The part screen's address stays in the Demo and in this visit. The
			// key is always one of the part constants, never the request value.
			p.Href = listHref + "&" + demoPartParam + "=" + p.Key
			parts = append(parts, p)
		}
		if name := view.CurrentName(); name != "" {
			vm.Nav.Title = name
			vm.Nav.BackHref = listHref
		}
		serveDemoPlace(w, tmpl, demoActivityTemplate, demoActivityViewModel{
			demoViewModel: vm,
			Parts:         parts,
			CurrentPart:   view.CurrentPart,
		})
	}
}

// demoPartParam is the query parameter that picks a part of My activity.
const demoPartParam = "part"

// demoActivityData reads the visitor's activity out of a world as the service
// shape the real page is built from. The neighbour votes that have fallen due
// are cast first, so the cards show the world as of now. Each report is read
// with the real Focus read, not the Nearby one: a report the visitor resolved or
// one that has faded is no longer in Nearby, but it stays on the visitor's own
// page, as it does for a real account. A report the read cannot show is left
// out. The votes carry the time they were cast.
func demoActivityData(ctx context.Context, world *demo.World) (service.ActivityPage, error) {
	if err := world.ApplyDue(ctx); err != nil {
		return service.ActivityPage{}, err
	}
	act := world.Activity()

	voted := make([]int64, 0, len(act.Confirmed)+len(act.Disputed))
	voted = append(voted, act.Confirmed...)
	voted = append(voted, act.Disputed...)
	votes, err := world.VisitorVotes(ctx, voted)
	if err != nil {
		return service.ActivityPage{}, err
	}

	// Amended 2026-11-17 (quick 261117-vuf): the visitor's practice reports are
	// no longer cards here, as the real page no longer lists your own posts, so
	// they are not read.
	page := service.ActivityPage{Posts: []service.ActivityReport{}, Votes: []service.VotedReport{}}
	for _, id := range voted {
		v, standing := votes[id]
		if !standing || !v.Value.ValidFor(service.VoteKindContent) {
			continue
		}
		ar, ok, err := demoActivityReport(ctx, world, id)
		if err != nil {
			return service.ActivityPage{}, err
		}
		if ok {
			page.Votes = append(page.Votes, service.VotedReport{ActivityReport: ar, Vote: v.Value, VotedAt: v.At})
		}
	}
	return page, nil
}

// demoActivityReport reads one report as the visitor sees it and shapes it as an
// Activity row. ok is false for a report the read cannot show.
func demoActivityReport(ctx context.Context, world *demo.World, id int64) (service.ActivityReport, bool, error) {
	res, err := world.Reports.Focus(ctx, service.FocusQuery{
		ReportID:        id,
		Latitude:        demo.PretendLat,
		Longitude:       demo.PretendLon,
		ViewerAccountID: demo.VisitorAccountID,
	})
	if err != nil {
		return service.ActivityReport{}, false, err
	}
	if res.Status != service.FocusOK || res.View == nil {
		return service.ActivityReport{}, false, nil
	}
	v := res.View

	row := sqlcgen.ReportsByAccountRow{
		ID:          v.ID,
		Category:    string(v.Category),
		Severity:    string(v.Severity),
		Description: v.Description,
		Latitude:    v.Latitude,
		Longitude:   v.Longitude,
		Geohash:     v.Geohash,
		CreatedAt:   v.CreatedAt,
		ExpiresAt:   v.ExpiresAt,
	}
	if v.ShelterCapacityStatus != nil {
		s := string(*v.ShelterCapacityStatus)
		row.ShelterCapacityStatus = &s
	}
	if v.ShelterHeadcount != nil {
		n := int32(*v.ShelterHeadcount)
		row.ShelterHeadcount = &n
	}
	return service.ActivityReport{
		Row:         row,
		Category:    v.Category,
		Severity:    v.Severity,
		Visibility:  v.Visibility,
		Reason:      v.VisibilityReason,
		Trust:       v.Trust,
		UndosLeft:   v.UndosLeft,
		Approximate: v.Approximate,
		WrittenAt:   v.WrittenAt,
	}, true, nil
}
