package handlers_test

// moderator_e2e_test.go is V-19: the moderator page, its 404 gate and (from
// Tasks 2 and 3) the queue, Find, Keep, Remove and the audit row. Helpers are
// prefixed mod so they cannot collide with another plan's.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/account"
	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	sqlcgen "pinalert/internal/store/sqlc"
)

// modBarSentence is the unofficial bar text on the moderator page with the
// default emergency numbers.
const modBarSentence = "Unofficial, not a government service. Emergency? Call 112 (older numbers 100, 101, 108 may also work). Disclaimer"

// modNoFollow stops a client following a redirect, so a 302 is seen as a 302.
func modNoFollow(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// modDo sends one request with the given method. POST, PUT and PATCH send the
// body {} as JSON; the other methods send none.
func modDo(t *testing.T, client *http.Client, method, url string) *http.Response {
	t.Helper()
	var body io.Reader
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		body = strings.NewReader("{}")
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("building %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// modSameAsUnknown sends the same method to path and to unknownPath with the
// same client and requires equal status (404), equal body bytes, equal headers
// once Date and Set-Cookie are dropped, and no Allow, Location or
// WWW-Authenticate header on either (D-08, correction 2, T-04-06).
func modSameAsUnknown(t *testing.T, client *http.Client, baseURL, method, path, unknownPath string) {
	t.Helper()
	read := func(p string) (int, []byte, http.Header) {
		resp := modDo(t, client, method, baseURL+p)
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("reading %s %s: %v", method, p, err)
		}
		h := resp.Header.Clone()
		h.Del("Date")
		h.Del("Set-Cookie")
		return resp.StatusCode, b, h
	}
	gotStatus, gotBody, gotHeader := read(path)
	wantStatus, wantBody, wantHeader := read(unknownPath)
	if gotStatus != http.StatusNotFound || wantStatus != http.StatusNotFound {
		t.Errorf("%s %s status = %d, unknown path status = %d, want both 404", method, path, gotStatus, wantStatus)
	}
	if !bytes.Equal(gotBody, wantBody) {
		t.Errorf("%s %s body = %q, unknown path body = %q, want equal", method, path, gotBody, wantBody)
	}
	if !reflect.DeepEqual(gotHeader, wantHeader) {
		t.Errorf("%s %s headers = %v, unknown path headers = %v, want equal", method, path, gotHeader, wantHeader)
	}
	for _, h := range []http.Header{gotHeader, wantHeader} {
		for _, name := range []string{"Allow", "Location", "Www-Authenticate"} {
			if v := h.Values(name); len(v) > 0 {
				t.Errorf("%s %s answered a %s header %v", method, path, name, v)
			}
		}
	}
}

// modStrip removes tags from an HTML fragment, unescapes entities and collapses
// whitespace.
var modTagPattern = regexp.MustCompile(`<[^>]*>`)

func modStrip(s string) string {
	s = modTagPattern.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// modRenderPage calls ModeratePage with the given config and request context
// and returns the recorder.
func modRenderPage(t *testing.T, cfg handlers.PageConfig, method string, withAccount bool) *httptest.ResponseRecorder {
	t.Helper()
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("parsing page templates: %v", err)
	}
	req := httptest.NewRequest(method, "/moderate", nil)
	if withAccount {
		req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 7, Email: "mod@example.com"}))
	}
	rec := httptest.NewRecorder()
	handlers.ModeratePage(tmpl, cfg)(rec, req)
	return rec
}

func TestModeratorPageRenders(t *testing.T) {
	rec := modRenderPage(t, handlers.PageConfig{AssetVersion: "test"}, http.MethodGet, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	body := rec.Body.String()

	once := func(what, needle string) {
		t.Helper()
		if got := strings.Count(body, needle); got != 1 {
			t.Errorf("%s: %q appears %d times, want exactly 1", what, needle, got)
		}
	}
	once("robots meta", `<meta name="robots" content="noindex">`)
	once("title", `<title>Pinalert: Moderator screen</title>`)
	once("top chrome", `class="top-chrome"`)
	once("h1", `<h1>Moderator screen</h1>`)
	for _, id := range []string{
		`id="mod-refresh"`, `id="mod-refresh-error"`, `id="mod-find-form"`, `id="mod-find"`,
		`id="mod-find-help"`, `id="mod-find-result"`, `<h2 id="mod-h-auto">`, `<h2 id="mod-h-people">`,
		`<h2 id="mod-h-pending">`, `id="toast"`,
	} {
		once("element", id)
	}
	for _, text := range []string{
		"Flagged by the automatic check", "Flagged by people", "Text check not finished",
		"Posted normally, but the text scored near the line. Newest first.",
		"People used Flag this report. Most flags first.",
		"The check couldn't finish. Serious reports in this list are already published. Waiting longest first.",
	} {
		once("heading or line", text)
	}
	if !strings.Contains(body, "mod@example.com") {
		t.Error("the account email is missing from the page")
	}

	start := strings.Index(body, `<aside class="disclaimer-bar"`)
	end := strings.Index(body, "</aside>")
	if start < 0 || end < start {
		t.Fatal("the unofficial bar is missing")
	}
	if got := modStrip(body[start : end+len("</aside>")]); got != modBarSentence {
		t.Errorf("bar text = %q, want %q", got, modBarSentence)
	}
	// The bar comes first in the top chrome, then the account header.
	if bar, header := strings.Index(body, `class="disclaimer-bar"`), strings.Index(body, `class="app-header"`); bar < 0 || header < bar {
		t.Errorf("the bar (%d) must come before the account header (%d)", bar, header)
	}

	for _, banned := range []string{"leaflet", "unpkg", "feed.css", "modal.css", "map.js"} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Errorf("the page mentions %q, but it loads no map or feed code", banned)
		}
	}

	custom := modRenderPage(t, handlers.PageConfig{Chrome: handlers.Chrome{Numbers: handlers.EmergencyNumbers{Primary: "999"}}}, http.MethodGet, true)
	if !strings.Contains(custom.Body.String(), "Call 999") {
		t.Error("a configured primary number does not reach the bar")
	}

	if rec := modRenderPage(t, handlers.PageConfig{}, http.MethodGet, false); rec.Code != http.StatusInternalServerError {
		t.Errorf("no account in context: status = %d, want 500", rec.Code)
	}
	if rec := modRenderPage(t, handlers.PageConfig{}, http.MethodPost, true); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status = %d, want 405", rec.Code)
	}
}

// modAllowList parses the list the way main.go does.
func modAllowList(t *testing.T, raw string) map[string]struct{} {
	t.Helper()
	set, err := service.ParseModeratorEmails(raw)
	if err != nil {
		t.Fatalf("ParseModeratorEmails(%q): %v", raw, err)
	}
	return set
}

func TestModeratorRoutesIndistinguishableFrom404(t *testing.T) {
	t.Run("empty_list_nobody", func(t *testing.T) {
		srv, mailer, _ := newE2EServerWith(t, nil)
		client := newVerifiedClient(t, srv, mailer, "mod@example.com")
		client.CheckRedirect = modNoFollow
		modSameAsUnknown(t, client, srv.URL, http.MethodGet, "/moderate", "/no-such-page")
	})

	srv, mailer, _ := newE2EServerWith(t, func(d *api.Deps, _ *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "Mod@Example.com")
	})

	anonymous := &http.Client{CheckRedirect: modNoFollow}

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("constructing cookie jar: %v", err)
	}
	unverified := &http.Client{Jar: jar, CheckRedirect: modNoFollow}
	if resp, err := unverified.Get(srv.URL + "/login"); err != nil {
		t.Fatalf("GET /login: %v", err)
	} else {
		resp.Body.Close()
	}

	reader := newVerifiedClient(t, srv, mailer, "reader@example.com")
	reader.CheckRedirect = modNoFollow

	moderator := newVerifiedClient(t, srv, mailer, "mod@example.com")
	moderator.CheckRedirect = modNoFollow

	all := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	rows := []struct {
		path    string
		methods []string
	}{
		{"/moderate", all},
		{"/api/moderation/queue", all},
		{"/api/moderation/reports/1", []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}},
		{"/api/moderation/reports/1/keep", []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut}},
		{"/api/moderation/reports/1/remove", []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut}},
		{"/api/moderation/reports/abc", []string{http.MethodGet, http.MethodPost}},
	}
	visitors := []struct {
		name   string
		client *http.Client
	}{
		{"anonymous", anonymous},
		{"unverified", unverified},
		{"verified_not_listed", reader},
	}
	for _, v := range visitors {
		for _, row := range rows {
			unknown := "/api/no-such-route"
			if row.path == "/moderate" {
				unknown = "/no-such-page"
			}
			for _, method := range row.methods {
				t.Run(fmt.Sprintf("%s/%s/%s", v.name, method, row.path), func(t *testing.T) {
					modSameAsUnknown(t, v.client, srv.URL, method, row.path, unknown)
				})
			}
		}
	}

	t.Run("moderator_gets_the_page", func(t *testing.T) {
		resp, err := moderator.Get(srv.URL + "/moderate")
		if err != nil {
			t.Fatalf("GET /moderate: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", resp.StatusCode, b)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", cc)
		}
		if !strings.Contains(string(b), "<title>Pinalert: Moderator screen</title>") {
			t.Error("the moderator page title is missing")
		}
	})

	t.Run("moderator_wrong_method_gets_405", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			resp := modDo(t, moderator, method, srv.URL+"/moderate")
			resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s /moderate status = %d, want 405", method, resp.StatusCode)
			}
		}
	})
}

// --- Task 2: the fixture, the queues and Find ---

// The six report texts of the fixture. Each is an ordinary sentence so the
// gibberish prefilter lets it through to the screener, and each is distinct so
// the stub can key on the exact trimmed text.
const (
	modTextReview      = "Water is rising fast near the old market"
	modTextFlagged     = "Road closed because of the fallen tree"
	modTextHeld        = "Power cut across the whole colony tonight"
	modTextSerious     = "Family trapped on the roof and needs a boat"
	modTextExpiredHeld = "Shelter at the school hall is now open"
	modTextPlain       = "Bridge on the highway is still passable"
)

// modUnfinishedTexts are the texts the stub answers with a transient provider
// error, so their checks stay pending.
var modUnfinishedTexts = map[string]bool{
	modTextHeld:        true,
	modTextSerious:     true,
	modTextExpiredHeld: true,
}

// modBorderlineScores are the stub scores of the borderline text.
var modBorderlineScores = map[string]float64{"harassment": 0.5, "hate": 0.1, "violence": 0.3}

// modStubScreener implements service.TextScreener keyed on the exact trimmed
// text.
type modStubScreener struct{}

func (modStubScreener) Screen(_ context.Context, text string) (moderation.Result, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == modTextReview:
		return moderation.Result{Model: moderation.DefaultModel, Scores: modBorderlineScores}, nil
	case modUnfinishedTexts[text]:
		return moderation.Result{}, &moderation.CallError{Status: 503, Transient: true}
	}
	return moderation.Result{Model: moderation.DefaultModel, Scores: map[string]float64{"harassment": 0.01, "hate": 0.01}}, nil
}

// modFixture is one server with the six reports posted and the clients.
type modFixture struct {
	srv         *httptest.Server
	pool        *pgxpool.Pool
	author      *http.Client
	readerB     *http.Client
	readerC     *http.Client
	moderator   *http.Client
	moderatorID int64
	ids         map[string]int64
}

const (
	modLat = 17.385
	modLon = 78.4867
)

// modSetup builds the server (screener stub, moderator list from a mixed case
// string), the four clients and the six reports, then has readers B and C flag
// the flagged report with spam and abusive.
func modSetup(t *testing.T) modFixture {
	t.Helper()
	srv, mailer, pool := newE2EServerWith(t, func(d *api.Deps, q *sqlcgen.Queries) {
		d.ModeratorEmails = modAllowList(t, "Mod@Example.com")
		d.Reports = service.NewReportService(q, service.WithScreener(modStubScreener{}))
	})
	fx := modFixture{
		srv:       srv,
		pool:      pool,
		author:    newVerifiedClient(t, srv, mailer, "author@example.com"),
		readerB:   newVerifiedClient(t, srv, mailer, "reader-b@example.com"),
		readerC:   newVerifiedClient(t, srv, mailer, "reader-c@example.com"),
		moderator: newVerifiedClient(t, srv, mailer, "mod@example.com"),
		ids:       map[string]int64{},
	}
	if err := pool.QueryRow(context.Background(), `SELECT id FROM accounts WHERE email = $1`, "mod@example.com").Scan(&fx.moderatorID); err != nil {
		t.Fatalf("reading the moderator account id: %v", err)
	}

	post := func(name, text string, critical bool) {
		body := floodReportBody(modLat, modLon)
		body["description"] = text
		if critical {
			body["severity"] = "critical"
		}
		fx.ids[name] = submitReportGetID(t, fx.author, srv.URL, body)
	}
	post("review", modTextReview, false)
	post("flagged", modTextFlagged, false)
	post("held", modTextHeld, false)
	post("serious", modTextSerious, true)
	post("expiredHeld", modTextExpiredHeld, false)
	expireAfter := fx.ids["expiredHeld"]
	post("plain", modTextPlain, false)
	expireReport(t, pool, expireAfter)

	for client, reason := range map[*http.Client]string{fx.readerB: "spam", fx.readerC: "abusive"} {
		if status := modFlag(t, client, srv.URL, fx.ids["flagged"], reason); status != http.StatusCreated {
			t.Fatalf("flagging as %s: status %d, want 201", reason, status)
		}
	}
	return fx
}

// modFlag posts a flag and returns the status.
func modFlag(t *testing.T, client *http.Client, baseURL string, id int64, reason string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"reason": reason})
	resp, err := client.Post(baseURL+"/api/reports/"+strconv.FormatInt(id, 10)+"/flag", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("flagging report %d: %v", id, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

// modQueueWire mirrors GET /api/moderation/queue. Rows are generic maps so a
// test can assert that a key is absent.
type modQueueWire struct {
	Review  []map[string]any `json:"review"`
	Flagged []map[string]any `json:"flagged"`
	Pending []map[string]any `json:"pending"`
	Counts  struct {
		Review  int64 `json:"review"`
		Flagged int64 `json:"flagged"`
		Pending int64 `json:"pending"`
	} `json:"counts"`
}

// modGetQueue fetches the queue and returns the status, the headers, the decoded
// answer and the raw body.
func modGetQueue(t *testing.T, client *http.Client, baseURL string) (int, http.Header, modQueueWire, []byte) {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/moderation/queue")
	if err != nil {
		t.Fatalf("GET queue: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var q modQueueWire
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, &q); err != nil {
			t.Fatalf("decoding the queue: %v: %s", err, raw)
		}
	}
	return resp.StatusCode, resp.Header, q, raw
}

// modFindWire mirrors GET /api/moderation/reports/{id}.
type modFindWire struct {
	Report map[string]any `json:"report"`
	Lists  []string       `json:"lists"`
	Error  struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	} `json:"error"`
}

func modFind(t *testing.T, client *http.Client, baseURL string, id int64) (int, modFindWire) {
	t.Helper()
	resp, err := client.Get(baseURL + "/api/moderation/reports/" + strconv.FormatInt(id, 10))
	if err != nil {
		t.Fatalf("GET report %d: %v", id, err)
	}
	defer resp.Body.Close()
	var out modFindWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding report %d: %v", id, err)
	}
	return resp.StatusCode, out
}

// modHas reports whether the row has the key.
func modHas(row map[string]any, key string) bool {
	_, ok := row[key]
	return ok
}

// modIDs lists the ids of rows.
func modIDs(rows []map[string]any) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, int64(r["id"].(float64)))
	}
	return out
}

// modWalkKeys appends every object key found anywhere in v.
func modWalkKeys(v any, keys *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			*keys = append(*keys, k)
			modWalkKeys(child, keys)
		}
	case []any:
		for _, child := range x {
			modWalkKeys(child, keys)
		}
	}
}

func TestModeratorQueues(t *testing.T) {
	fx := modSetup(t)
	base := fx.srv.URL

	status, header, q, raw := modGetQueue(t, fx.moderator, base)
	if status != http.StatusOK {
		t.Fatalf("queue status = %d: %s", status, raw)
	}
	if cc := header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if q.Counts.Review != 1 || q.Counts.Flagged != 1 || q.Counts.Pending != 2 {
		t.Errorf("counts = %+v, want review 1, flagged 1, pending 2", q.Counts)
	}
	if got := modIDs(q.Review); !reflect.DeepEqual(got, []int64{fx.ids["review"]}) {
		t.Errorf("review ids = %v, want [%d]", got, fx.ids["review"])
	}
	if got := modIDs(q.Flagged); !reflect.DeepEqual(got, []int64{fx.ids["flagged"]}) {
		t.Errorf("flagged ids = %v, want [%d]", got, fx.ids["flagged"])
	}
	if got := modIDs(q.Pending); !reflect.DeepEqual(got, []int64{fx.ids["held"], fx.ids["serious"]}) {
		t.Errorf("pending ids = %v, want [%d %d] (expiredHeld absent)", got, fx.ids["held"], fx.ids["serious"])
	}

	if len(q.Review) == 1 {
		row := q.Review[0]
		scores, _ := row["scores"].(map[string]any)
		if len(scores) != len(modBorderlineScores) {
			t.Errorf("review scores = %v, want the stub map", row["scores"])
		}
		for k, want := range modBorderlineScores {
			if got, _ := scores[k].(float64); got != want {
				t.Errorf("score %s = %v, want %v", k, scores[k], want)
			}
		}
		if row["model"] != moderation.DefaultModel || row["moderation"] != "clear" || row["view"] != "active" || row["visibility_word"] != "Showing to everyone" {
			t.Errorf("review row = %v", row)
		}
	}
	if len(q.Flagged) == 1 {
		row := q.Flagged[0]
		flags, _ := row["flags"].(map[string]any)
		if flags["total"] != float64(2) {
			t.Errorf("flags total = %v, want 2", flags["total"])
		}
		wantReasons := []any{
			map[string]any{"reason": "spam", "label": "Spam or gibberish", "count": float64(1)},
			map[string]any{"reason": "abusive", "label": "Abusive or hateful", "count": float64(1)},
		}
		if !reflect.DeepEqual(flags["reasons"], wantReasons) {
			t.Errorf("flag reasons = %v, want %v", flags["reasons"], wantReasons)
		}
		if modHas(row, "scores") {
			t.Error("a flagged row with no review check must carry no scores key")
		}
	}
	if len(q.Pending) == 2 {
		held, serious := q.Pending[0], q.Pending[1]
		if held["moderation"] != "pending" || held["serious"] != false || held["view"] != "none" || modHas(held, "visibility_word") {
			t.Errorf("held row = %v, want pending, not serious, view none, no visibility_word", held)
		}
		if serious["serious"] != true || serious["view"] != "active" || serious["visibility_word"] != "Showing to everyone" {
			t.Errorf("serious row = %v, want serious, view active, Showing to everyone", serious)
		}
	}

	// No identity anywhere in the answer (T-04-04).
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding raw queue: %v", err)
	}
	var keys []string
	modWalkKeys(decoded, &keys)
	for _, k := range keys {
		lower := strings.ToLower(k)
		for _, banned := range []string{"email", "session", "account", "flagger"} {
			if strings.Contains(lower, banned) {
				t.Errorf("the queue carries a key %q that names an identity", k)
			}
		}
	}
	for _, addr := range []string{"author@example.com", "reader-b@example.com", "reader-c@example.com", "mod@example.com"} {
		if strings.Contains(string(raw), addr) {
			t.Errorf("the queue body contains %s", addr)
		}
	}

	// Find.
	for name, want := range map[string][]string{"review": {"review"}, "held": {"pending"}, "expiredHeld": {}, "plain": {}} {
		status, out := modFind(t, fx.moderator, base, fx.ids[name])
		if status != http.StatusOK {
			t.Errorf("Find %s status = %d", name, status)
			continue
		}
		if out.Lists == nil || !reflect.DeepEqual(out.Lists, want) {
			if !(len(want) == 0 && out.Lists != nil && len(out.Lists) == 0) {
				t.Errorf("Find %s lists = %v, want %v", name, out.Lists, want)
			}
		}
		if name == "review" && !modHas(out.Report, "scores") {
			t.Error("Find review must carry scores")
		}
		if (name == "expiredHeld" || name == "plain") && modHas(out.Report, "scores") {
			t.Errorf("Find %s must carry no scores", name)
		}
	}
	status, out := modFind(t, fx.moderator, base, 999999)
	if status != http.StatusNotFound || out.Error.Field != "id" || out.Error.Message != service.ModNotFoundMessage {
		t.Errorf("Find 999999 = %d %+v, want 404 field id with the not found message", status, out.Error)
	}
}
