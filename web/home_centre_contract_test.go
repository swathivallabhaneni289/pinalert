package web

// home_centre_contract_test.go pins the page side of plan 03.4-33 task 2: when
// the device location is not available, the map starts at the centre the server
// put in data-fallback-lat and data-fallback-lon, at the zoom the server named
// in data-fallback-zoom when it started the page at a home area centre (13, about
// 7 km across a phone), and at the city zoom 11 otherwise.

import (
	"regexp"
	"strings"
	"testing"
)

func TestFallbackZoomComesFromThePage(t *testing.T) {
	app := p421ReadStatic(t, "js/app.js")
	p13MustContain(t, "app.js", app, "fallbackZoom: readNumberAttr('data-fallback-zoom', 11)")

	mapJS := p23CodeOf(t, "static/js/map.js")
	centre := jsFunctionBody(t, mapJS, "centerOnVisitor()")
	p13MustContain(t, "centerOnVisitor", centre,
		"var fallbackZoom = Pinalert.config.fallbackZoom;",
		"boot(fallbackLat, fallbackLon, fallbackZoom, false)")
	back := jsFunctionBody(t, mapJS, "recenterOnReader()")
	p13MustContain(t, "recenterOnReader", back,
		"Pinalert.config.fallbackLat, Pinalert.config.fallbackLon, Pinalert.config.fallbackZoom, false")
	// The city zoom is no longer a literal in either fallback path; a device
	// fix keeps its own zoom 13.
	if regexp.MustCompile(`fallback(Lat|Lon)[^;\n]*, 11,`).MatchString(mapJS) {
		t.Errorf("map.js still names the literal city zoom 11 on a fallback path")
	}
	p13MustContain(t, "centerOnVisitor", centre, "boot(pos.coords.latitude, pos.coords.longitude, 13, true)")
}

func TestFallbackZoomAttributeIsOnTheMapPageOnly(t *testing.T) {
	index := stripReadTemplate(t, "index.html.tmpl")
	p13MustContain(t, "index.html.tmpl", index, `{{if .FallbackZoom}} data-fallback-zoom="{{.FallbackZoom}}"{{end}}`)
	if n := strings.Count(index, "data-fallback-zoom"); n != 1 {
		t.Errorf("index.html.tmpl holds data-fallback-zoom %d times, want 1", n)
	}
	low := stripReadTemplate(t, "low.html.tmpl")
	p13MustNotContain(t, "low.html.tmpl", low, "data-fallback-zoom", "FallbackZoom")
}
