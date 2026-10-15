package web

// written_line_contract_test.go pins D-19 and P4-A8 in the browser modules: the
// "Written 4 hours ago" line of a report that reached the server late comes from
// one formatter, Pinalert.writtenLine in app.js, and is shown on the feed row
// and in the map popup. It reads source text. It does not run a browser, so the
// look at 320, 360 and 390 px wide is a human check (V-24).

import (
	"io/fs"
	"strings"
	"testing"
)

func p17ReadStaticJS(t *testing.T, name string) string {
	t.Helper()
	raw, err := fs.ReadFile(StaticFS, "static/js/"+name)
	if err != nil {
		t.Fatalf("reading static/js/%s: %v", name, err)
	}
	return string(raw)
}

// p17WantAll fails for every token missing from body.
func p17WantAll(t *testing.T, where, body string, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		if !strings.Contains(body, tok) {
			t.Errorf("%s: missing %q", where, tok)
		}
	}
}

// p17WantNone fails for every token present in body.
func p17WantNone(t *testing.T, where, body string, tokens ...string) {
	t.Helper()
	for _, tok := range tokens {
		if strings.Contains(body, tok) {
			t.Errorf("%s: must not contain %q", where, tok)
		}
	}
}

// p17WantInOrder fails unless each token occurs after the one before it.
func p17WantInOrder(t *testing.T, where, body string, tokens ...string) {
	t.Helper()
	at := 0
	for _, tok := range tokens {
		i := strings.Index(body[at:], tok)
		if i == -1 {
			t.Errorf("%s: %q is missing or out of order", where, tok)
			return
		}
		at += i + len(tok)
	}
}

func TestWrittenLineFormatter(t *testing.T) {
	app := p17ReadStaticJS(t, "app.js")
	feed := p17ReadStaticJS(t, "feed.js")
	mapJS := p17ReadStaticJS(t, "map.js")

	t.Run("app.js", func(t *testing.T) {
		body := stripJSComments(jsFunctionBody(t, app, "writtenLine(report)"))
		p17WantAll(t, "app.js writtenLine", body, "report.written_at", "'Written '", "relativeTime(", "return ''")
		p17WantAll(t, "app.js", app, "writtenLine: writtenLine")

		rel := jsFunctionBody(t, app, "relativeTime(iso)")
		p17WantAll(t, "app.js relativeTime", rel, "' min ago'", "' hour ago'", "' hours ago'", "' day ago'")
		p17WantNone(t, "app.js relativeTime", rel, "Written")

		p17WantNone(t, "app.js ageStage", stripJSComments(jsFunctionBody(t, app, "ageStage(report)")), "written_at")
	})

	t.Run("one formatter", func(t *testing.T) {
		var holders []string
		err := fs.WalkDir(StaticFS, "static/js", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".js") {
				return nil
			}
			raw, err := fs.ReadFile(StaticFS, path)
			if err != nil {
				return err
			}
			if strings.Contains(stripJSComments(string(raw)), "'Written '") {
				holders = append(holders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking static/js: %v", err)
		}
		if len(holders) != 1 || holders[0] != "static/js/app.js" {
			t.Fatalf("the literal 'Written ' must live in app.js only, found in %v", holders)
		}
	})

	t.Run("feed.js", func(t *testing.T) {
		update := stripJSComments(jsFunctionBody(t, feed, "updateWrittenLine(el, report)"))
		p17WantAll(t, "feed.js updateWrittenLine", update, "Pinalert.writtenLine(report)", "Pinalert.setText(", ".hidden =")
		for _, fn := range []string{"updateRow(row, report)", "refreshAgeAndTime()"} {
			p17WantAll(t, "feed.js "+fn, stripJSComments(jsFunctionBody(t, feed, fn)), "updateWrittenLine(")
		}

		create := stripJSComments(jsFunctionBody(t, feed, "createRow(id)"))
		p17WantAll(t, "feed.js createRow", create, "written.className = 'report-row__meta'", "written.hidden = true", "written: written")
		p17WantInOrder(t, "feed.js createRow", create, "body.appendChild(meta);", "body.appendChild(written);", "body.appendChild(away);")

		for _, fn := range []string{"buildMetaText(report)", "sortReports(reports)", "severityRank(report)"} {
			p17WantNone(t, "feed.js "+fn, stripJSComments(jsFunctionBody(t, feed, fn)), "written")
		}
	})

	t.Run("map.js", func(t *testing.T) {
		build := stripJSComments(jsFunctionBody(t, mapJS, "buildPopupCard(report)"))
		p17WantAll(t, "map.js buildPopupCard", build, "writtenLine.className = 'popup-card__meta-line'", "writtenLine.hidden = true", "writtenLine: writtenLine")
		p17WantInOrder(t, "map.js buildPopupCard", build,
			"metaLines.appendChild(metaLine);", "metaLines.appendChild(writtenLine);", "metaLines.appendChild(shelterLine);")

		update := stripJSComments(jsFunctionBody(t, mapJS, "updatePopupCard(card, report)"))
		p17WantAll(t, "map.js updatePopupCard", update, "Pinalert.writtenLine(report)", "Pinalert.setText(card.writtenLine", "card.writtenLine.hidden =")

		p17WantNone(t, "map.js popupStateClasses", stripJSComments(jsFunctionBody(t, mapJS, "popupStateClasses(report)")), "written")
	})
}
