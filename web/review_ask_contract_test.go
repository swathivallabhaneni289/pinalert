package web

// review_ask_contract_test.go pins the words and the wiring of quick
// 261118-rz6 (owner 2026-11-18): the author's box for a removed report, its
// bell and full list rows, the server lines it shows, the Policy sentence, and
// no en or em dash in any file the change touched. The moderator screen's own
// words are pinned in moderate_contract_test.go.

import (
	"os"
	"strings"
	"testing"
)

func rzReadRepo(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}

// TestReviewAskBoxMarkup: the box is the Log out box with exactly the words
// the owner agreed, a note of at most 200 characters with no placeholder, and
// it is a child of body on every page that loads updates.js, never inside the
// bell dropdown.
func TestReviewAskBoxMarkup(t *testing.T) {
	box := p421ReadTemplate(t, "review_ask_box.html.tmpl")
	p421InOrder(t, "the box", box,
		`<dialog id="review-ask-box" class="logout-confirm review-ask" aria-labelledby="review-ask-title">`,
		`id="review-ask-title" tabindex="-1">Removed</h2>`,
		`id="review-ask-why" hidden></p>`,
		`<label for="review-ask-note">Note (optional)</label>`,
		`<textarea id="review-ask-note" maxlength="200"`,
		`id="review-ask-asked" tabindex="-1" hidden>Asked.</p>`,
		`id="review-ask-none" tabindex="-1" hidden>No asks left.</p>`,
		`id="review-ask-error" role="alert" data-failed="Couldn't send. Try again." hidden></p>`,
		`id="review-ask-close">Close</button>`,
		`id="review-ask-send" hidden>Ask for review</button>`,
		`</dialog>`)
	for _, banned := range []string{"placeholder", "flag", "moderator said", "Policy"} {
		if strings.Contains(strings.SplitN(box, "*/}}", 2)[1], banned) {
			t.Errorf("the box holds %q", banned)
		}
	}
	for _, page := range []string{"index.html.tmpl", "profile.html.tmpl", "settings.html.tmpl", "alerts.html.tmpl", "person.html.tmpl"} {
		tmpl := p421ReadTemplate(t, page)
		if n := strings.Count(tmpl, `{{template "review_ask_box.html.tmpl" .}}`); n != 1 {
			t.Errorf("%s includes the box %d times, want 1", page, n)
		}
		p421InOrder(t, page+" box before the bottom bar", tmpl, `{{template "review_ask_box.html.tmpl" .}}`, `{{template "app_bottombar" .}}`)
	}
	if strings.Contains(p421ReadTemplate(t, "app_bar.html.tmpl"), "review-ask") {
		t.Error("the box sits inside the app bar, where the bell dropdown could clip it")
	}
}

// TestReviewAskRowsAndScript: the full list's box row is a button carrying the
// checked id and the new mark while unread; updates.js reads and sends the box
// through its one request helper, never opens the Policy, and writes text
// only.
func TestReviewAskRowsAndScript(t *testing.T) {
	profile := p421ReadTemplate(t, "profile.html.tmpl")
	for _, want := range []string{
		`{{if and .Unread (or .Href .OwnPlain .ReviewReportID)}} urow--new{{end}}`,
		`{{else if .ReviewReportID}}<button type="button" class="urow__link urow__box" data-review-ask="{{.ReviewReportID}}" aria-haspopup="dialog" aria-label="{{.Label}}">`,
		`{{else if .ReviewReportID}}</button>`,
	} {
		if !strings.Contains(profile, want) {
			t.Errorf("profile.html.tmpl lacks %q", want)
		}
	}
	code := stripJSComments(p421ReadStatic(t, "js/updates.js"))
	for _, want := range []string{
		"'/api/reports/'", "'/review-ask'", "review_report_id", "data-review-ask", "showModal()", "ID_PATTERN.test(id)",
		"JSON.stringify({ note: ask.note.value })", "'can_ask'", "'asked'", "'no_asks_left'",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("updates.js lacks %q", want)
		}
	}
	for _, banned := range []string{"'/policy'", "LINK_POLICY", "innerHTML"} {
		if strings.Contains(code, banned) {
			t.Errorf("updates.js still holds %q", banned)
		}
	}
	css := p421ReadStatic(t, "css/updates.css")
	for _, want := range []string{"button.bell-row,", "button.urow__link {", ".review-ask__note textarea {"} {
		if !strings.Contains(css, want) {
			t.Errorf("updates.css lacks %q", want)
		}
	}
}

// TestReviewAskServerWords: every new line the server sends is the agreed one,
// and the Policy says, in one sentence, that a removed report says why and its
// author can ask for a review from the bell, up to 3 times.
func TestReviewAskServerWords(t *testing.T) {
	copyGo := rzReadRepo(t, "../internal/service/updatescopy.go")
	for _, want := range []string{
		`OwnRestoredLine     = "Your report is back."`,
		`OwnStaysRemovedLine = "Your report stays removed."`,
		`OwnRemovedLine = "Your report was removed after review."`,
	} {
		if !strings.Contains(copyGo, want) {
			t.Errorf("updatescopy.go lacks %s", want)
		}
	}
	modGo := rzReadRepo(t, "../internal/service/moderation.go")
	for _, want := range []string{`return "Spam"`, `return "Abusive"`, `return "False"`, `return "Dangerous"`, `return "Other"`, `return "Text check"`, `"Why: " + label`, `RemovalReasonMessage = "Pick a reason."`} {
		if !strings.Contains(modGo, want) {
			t.Errorf("moderation.go lacks %s", want)
		}
	}
	askGo := rzReadRepo(t, "../internal/service/reviewask.go")
	for _, want := range []string{
		`ReviewAskOpenMessage = "You already asked. Wait for the answer."`,
		`ReviewAskNoneLeftMessage = "No asks left."`,
		`ReviewAskNoteTooLongMessage = "Keep the note to 200 characters or fewer."`,
		`const ReviewAskLimit = 3`, `const ReviewAskNoteMaxChars = 200`,
	} {
		if !strings.Contains(askGo, want) {
			t.Errorf("reviewask.go lacks %s", want)
		}
	}
	legal := p421ReadTemplate(t, "legal_body.html.tmpl")
	const sentence = "The bell also says why it was removed, and its author can ask for a review there, up to 3 times."
	if n := strings.Count(legal, sentence); n != 1 {
		t.Errorf("the Policy sentence appears %d times, want 1", n)
	}
	p421InOrder(t, "the Policy moderation part", legal, "<h3>What moderators can do</h3>", "whether a moderator removed it or the late automatic check did. "+sentence+"</p>")
}

// TestReviewAskNoDashes: no en or em dash in any file quick 261118-rz6 wrote or
// changed.
func TestReviewAskNoDashes(t *testing.T) {
	for _, path := range []string{
		"templates/review_ask_box.html.tmpl", "templates/moderate.html.tmpl", "templates/profile.html.tmpl",
		"templates/legal_body.html.tmpl", "static/js/updates.js", "static/js/moderate.js",
		"static/css/updates.css", "static/css/moderate.css", "moderate_remove_test.mjs", "updates_test.mjs",
		"../internal/store/migrations/00027_add_removal_reasons_and_review_asks.sql",
		"../internal/store/queries/review_asks.sql", "../internal/store/queries/moderation.sql",
		"../internal/service/reviewask.go", "../internal/service/moderation.go", "../internal/service/updatescopy.go",
		"../internal/service/review_ask_test.go", "../internal/service/push_restore_db_test.go",
		"../internal/api/handlers/reviewask.go", "../internal/api/handlers/moderation.go",
		"../internal/api/handlers/review_ask_e2e_test.go", "../docs/MODERATION.md",
	} {
		text := rzReadRepo(t, path)
		for _, dash := range []string{"–", "—"} {
			if strings.Contains(text, dash) {
				t.Errorf("%s holds the dash %q", path, dash)
			}
		}
	}
}
