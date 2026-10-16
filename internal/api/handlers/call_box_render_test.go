package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPostWindowCallBoxRendersConfiguredNumber (Phase 3.4 plan 12, D-24a,
// T-34-53): the Call 112 box of the Post a report window is one tel link built
// from the numbers setting. With the default it dials 112; with another
// primary number it dials that number in the link, the line and the button,
// and the page never holds a Go template rewrite marker.
func TestPostWindowCallBoxRendersConfiguredNumber(t *testing.T) {
	tmpl, err := ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	for _, tc := range []struct {
		name   string
		chrome Chrome
		number string
	}{
		{"default", Chrome{}, "112"},
		{"configured", Chrome{Numbers: EmergencyNumbers{Primary: "108"}}, "108"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Page(tmpl, PageConfig{Chrome: tc.chrome})(rec, newPageTestRequest())
			body := rec.Body.String()

			start := strings.Index(body, `<a id="call-112-box"`)
			if start == -1 {
				t.Fatalf("the page has no Call 112 box")
			}
			box := body[start : start+strings.Index(body[start:], "</a>")]
			for _, want := range []string{
				`href="tel:` + tc.number + `"`,
				`Call ` + tc.number + ` first, then post here.`,
				`id="call-112-action">Call ` + tc.number + `</span>`,
				`Someone in danger right now?`,
				` hidden>`,
			} {
				if !strings.Contains(box, want) {
					t.Errorf("the Call 112 box lacks %q:\n%s", want, box)
				}
			}
			if n := strings.Count(body, `href="tel:`+tc.number+`"`); n < 1 {
				t.Errorf("no tel link for %s on the page", tc.number)
			}
			if strings.Contains(body, "ZgotmplZ") {
				t.Errorf("the page holds ZgotmplZ, so the tel target was rewritten")
			}
			if !strings.Contains(body, `id="happening-fields"`) || !strings.Contains(body, `id="happening-group"`) {
				t.Errorf("the page has no What is happening? fieldset")
			}
		})
	}
}
