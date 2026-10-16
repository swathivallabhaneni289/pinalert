package handlers_test

import (
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"pinalert/internal/account"
	"pinalert/internal/api/handlers"
	"pinalert/internal/service"
)

var (
	legalTagPattern   = regexp.MustCompile(`(?s)<[^>]*>`)
	legalSpacePattern = regexp.MustCompile(`\s+`)
	legalH2Pattern    = regexp.MustCompile(`(?s)<h2[^>]*>(.*?)</h2>`)
)

// legalText removes tags, unescapes entities (html/template writes an
// apostrophe as an entity) and collapses whitespace runs to one space.
func legalText(t *testing.T, body string) string {
	t.Helper()
	text := legalTagPattern.ReplaceAllString(body, "")
	text = html.UnescapeString(text)
	return strings.TrimSpace(legalSpacePattern.ReplaceAllString(text, " "))
}

// legalRender serves one legal page through the handler.
func legalRender(t *testing.T, cfg handlers.AuthConfig, page string, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		t.Fatalf("ParsePageTemplate: %v", err)
	}
	if r == nil {
		r = httptest.NewRequest(http.MethodGet, "/"+page, nil)
	}
	rec := httptest.NewRecorder()
	handlers.Legal(tmpl, cfg, page).ServeHTTP(rec, r)
	return rec
}

// legalH2s returns the visible h2 texts in order.
func legalH2s(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	for _, m := range legalH2Pattern.FindAllStringSubmatch(body, -1) {
		out = append(out, legalText(t, m[1]))
	}
	return out
}

func legalEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var legalDisclaimerH2s = []string{
	"Pinalert is not a substitute for emergency services",
	"Emergency numbers",
	"How to read what Pinalert shows",
	"Reports can be wrong",
	"Reports expire",
}

func TestLegalPagesRender(t *testing.T) {
	cfg := handlers.AuthConfig{AssetVersion: "test"}
	rec := legalRender(t, cfg, handlers.LegalDisclaimerPage, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<h1>Disclaimer</h1>") {
		t.Errorf("missing h1 Disclaimer")
	}
	if got := legalH2s(t, body); !legalEqual(got, legalDisclaimerH2s) {
		t.Errorf("h2 sequence = %q, want %q", got, legalDisclaimerH2s)
	}
	for _, href := range []string{"/static/css/main.css?v=test", "/static/css/chrome.css?v=test", "/static/js/theme.js?v=test"} {
		if n := strings.Count(body, href); n != 1 {
			t.Errorf("%s appears %d times, want 1", href, n)
		}
	}
	if n := strings.Count(body, "<script"); n != 1 {
		t.Errorf("<script appears %d times, want 1 (theme.js only)", n)
	}
	if !strings.Contains(body, `href="/disclaimer" aria-current="page"`) {
		t.Errorf("the bar Disclaimer link must carry aria-current=page")
	}
	if n := strings.Count(body, `<a class="legal-back" href="/">Back to Pinalert</a>`); n != 1 {
		t.Errorf("top Back to Pinalert link appears %d times, want 1", n)
	}
	if n := strings.Count(body, `href="/">Back to Pinalert</a>`); n != 2 {
		t.Errorf("Back to Pinalert links = %d, want 2", n)
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Errorf("page holds ZgotmplZ")
	}

	if n := strings.Count(body, "/static/css/legal.css?v=test"); n != 1 {
		t.Errorf("disclaimer legal.css appears %d times, want 1", n)
	}
	if !strings.Contains(body, `<a href="/policy">Policy</a>`) {
		t.Errorf("disclaimer footer lacks the Policy link")
	}

	bad := legalRender(t, cfg, "nope", nil)
	if bad.Code != http.StatusInternalServerError || strings.TrimSpace(bad.Body.String()) != "internal server error" {
		t.Errorf("unknown page = %d %q, want 500 internal server error", bad.Code, bad.Body.String())
	}

	// The Policy page.
	prec := legalRender(t, cfg, handlers.LegalPolicyPage, nil)
	if prec.Code != http.StatusOK {
		t.Fatalf("policy status = %d, want 200", prec.Code)
	}
	if got := prec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("policy Content-Type = %q", got)
	}
	if got := prec.Header().Get("Cache-Control"); got != "private, max-age=300" {
		t.Errorf("policy Cache-Control = %q", got)
	}
	pb := prec.Body.String()
	if !strings.Contains(pb, "<h1>Policy</h1>") {
		t.Errorf("missing h1 Policy")
	}
	if got := legalH2s(t, pb); !legalEqual(got, legalPolicyH2sWithoutContact) {
		t.Errorf("policy h2 sequence = %q, want %q", got, legalPolicyH2sWithoutContact)
	}
	ptext := legalText(t, pb)
	if !strings.Contains(ptext, "Rules, privacy and terms") {
		t.Errorf("policy subtitle missing")
	}
	for _, href := range []string{"/static/css/main.css?v=test", "/static/css/chrome.css?v=test", "/static/css/legal.css?v=test", "/static/js/theme.js?v=test"} {
		if n := strings.Count(pb, href); n != 1 {
			t.Errorf("policy %s appears %d times, want 1", href, n)
		}
	}
	if n := strings.Count(pb, "<script"); n != 1 {
		t.Errorf("policy <script appears %d times, want 1", n)
	}
	if !strings.Contains(pb, `href="/disclaimer">Disclaimer</a>`) || strings.Contains(pb, "aria-current") {
		t.Errorf("policy bar link must not carry aria-current")
	}
	if n := strings.Count(pb, `href="/">Back to Pinalert</a>`); n != 2 {
		t.Errorf("policy Back to Pinalert links = %d, want 2", n)
	}
	if !strings.Contains(pb, `<p class="legal-links">`) || strings.Count(pb, `href="/disclaimer"`) != 2 {
		t.Errorf("policy needs the Disclaimer link in the bar and in the footer")
	}
	updated := regexp.MustCompile(`(?s)<p class="legal-updated">(.*?)</p>`).FindStringSubmatch(pb)
	if updated == nil {
		t.Fatalf("policy lacks the Last updated line")
	}
	lastUpdated := regexp.MustCompile(`^Last updated [1-9][0-9]? (January|February|March|April|May|June|July|August|September|October|November|December) 20[0-9]{2}$`)
	if got := legalText(t, updated[1]); !lastUpdated.MatchString(got) {
		t.Errorf("Last updated line = %q", got)
	}
	cookies := regexp.MustCompile(`(?s)Cookies and storage on your device</h2>.*?</ul>`).FindString(pb)
	if !strings.Contains(legalText(t, cookies), "before you sign in") {
		t.Errorf("the cookie list must say the session cookie is set before you sign in")
	}
	if strings.Contains(pb, "tel:") || strings.Contains(pb, "ZgotmplZ") {
		t.Errorf("policy holds a tel link or ZgotmplZ")
	}
	for _, want := range []string{
		"For anything serious, such as a crime or a threat to someone's safety, contact the police directly. In an emergency call 112. Pinalert doesn't contact the police for you.",
		"Browsers can delete saved data, so a report waiting to send isn't guaranteed to survive. Send it as soon as you have a signal.",
		"Nothing listed here is deleted on a schedule today",
		// Review finding IN-W01 (Phase 4 web review): the three sentences
		// that now say what the background check stores and shows. A check
		// that finishes late removes the report instead of rejecting it, the
		// author then sees the same label a moderator removal shows, and the
		// worker stores verdict, scores and model for every late check.
		"If the text is clearly hateful, harassing or sexual, it is rejected and nothing is stored. If such text is only caught after the report was saved, because the check finished late, the report is removed instead: it stays stored but shows to no one. If the text is borderline, it is posted and a moderator may look at it.",
		`A removed report shows to no one. Its author sees "Removed after review" both when a moderator removed it and when the automatic check removed it after it was saved.`,
		"The result and scores of the automatic check, and which model gave them, without the report text: for borderline reports and for any report whose check finished after it was saved.",
	} {
		if !strings.Contains(ptext, want) {
			t.Errorf("policy text lacks %q", want)
		}
	}
	for _, banned := range []string{"threatening", "illegal"} {
		if strings.Contains(strings.ToLower(ptext), banned) {
			t.Errorf("policy text names %q", banned)
		}
	}
}

var legalPolicyH2sWithoutContact = []string{
	"What you can post",
	"How the automatic check works",
	"What moderators can do",
	"Limits",
	"What Pinalert keeps",
	"Cookies and storage on your device",
	"Emergencies and the police",
	"Terms",
	"Changes to this page",
}

func TestLegalPagesContactBlockOnlyWhenSet(t *testing.T) {
	set := handlers.AuthConfig{AssetVersion: "test", Chrome: handlers.Chrome{ContactEmail: "moderation@example.org"}}
	d := legalRender(t, set, handlers.LegalDisclaimerPage, nil).Body.String()
	dh := legalH2s(t, d)
	if len(dh) == 0 || dh[len(dh)-1] != "Contact" {
		t.Errorf("disclaimer last h2 = %q, want Contact", dh)
	}
	if !strings.Contains(legalText(t, d), "To report abuse or ask for something to be removed, write to moderation@example.org.") {
		t.Errorf("disclaimer contact sentence missing")
	}
	if !strings.Contains(d, `href="mailto:moderation@example.org"`) {
		t.Errorf("disclaimer mailto link missing")
	}
	p := legalRender(t, set, handlers.LegalPolicyPage, nil).Body.String()
	wantP := append(append([]string{}, legalPolicyH2sWithoutContact[:7]...), "Complaints and removal requests", "Terms", "Changes to this page")
	if got := legalH2s(t, p); !legalEqual(got, wantP) {
		t.Errorf("policy h2 with contact = %q, want %q", got, wantP)
	}
	if !strings.Contains(legalText(t, p), "To report abuse or ask for a report to be removed, write to moderation@example.org.") {
		t.Errorf("policy contact sentence missing")
	}
	if !strings.Contains(p, `href="mailto:moderation@example.org"`) {
		t.Errorf("policy mailto link missing")
	}
	if strings.Contains(strings.ToLower(legalText(t, p)), "within") {
		t.Errorf("policy promises a time")
	}

	unset := handlers.AuthConfig{AssetVersion: "test"}
	for _, page := range []string{handlers.LegalDisclaimerPage, handlers.LegalPolicyPage} {
		b := legalRender(t, unset, page, nil).Body.String()
		for _, banned := range []string{"Contact</h2>", "Complaints and removal requests", "To report abuse", "@", "mailto:"} {
			if strings.Contains(b, banned) {
				t.Errorf("%s page without an address holds %q", page, banned)
			}
		}
	}
}

func TestLegalPagesRenderNoAccountData(t *testing.T) {
	cfg := handlers.AuthConfig{AssetVersion: "test"}
	for _, page := range []string{handlers.LegalDisclaimerPage, handlers.LegalPolicyPage} {
		req := httptest.NewRequest(http.MethodGet, "/"+page, nil)
		req = req.WithContext(account.WithAccount(req.Context(), account.Account{ID: 7, Email: "legal-reader@example.com"}))
		body := legalRender(t, cfg, page, req).Body.String()
		for _, banned := range []string{"legal-reader@example.com", "app-header", "account-menu", "/logout", "data-account-id"} {
			if strings.Contains(body, banned) {
				t.Errorf("%s page holds %q", page, banned)
			}
		}
	}
}

func TestLegalPagesNumbersBlock(t *testing.T) {
	def := legalRender(t, handlers.AuthConfig{AssetVersion: "test"}, handlers.LegalDisclaimerPage, nil).Body.String()
	if n := strings.Count(def, `href="tel:112"`); n != 1 {
		t.Errorf("tel:112 links = %d, want 1", n)
	}
	if n := strings.Count(def, "tel:"); n != 1 {
		t.Errorf("tel: occurrences = %d, want 1", n)
	}
	if !strings.Contains(def, `aria-label="Call 112"`) {
		t.Errorf("call link needs the name Call 112")
	}
	if strings.Contains(def, "ZgotmplZ") {
		t.Errorf("page holds ZgotmplZ")
	}
	text := legalText(t, def)
	for _, want := range []string{
		"India's national emergency number for police, fire and ambulance.",
		"Older numbers for police (100), fire (101) and ambulance (108).",
		"Try 112 first.",
		"1070 and 1077",
		"State disaster management authorities",
		"Pinalert hasn't checked the numbers on that site.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text lacks %q", want)
		}
	}
	if n := strings.Count(text, "national"); n != 1 {
		t.Errorf("the word national appears %d times, want 1", n)
	}
	if !strings.Contains(def, `href="https://sachet.ndma.gov.in/HelpDesk"`) || !strings.Contains(def, `rel="noopener noreferrer"`) {
		t.Errorf("SACHET link missing or lacks rel")
	}

	custom := handlers.AuthConfig{AssetVersion: "test", Chrome: handlers.Chrome{Numbers: handlers.EmergencyNumbers{Primary: "999", Older: []string{"100", "101", "108"}}}}
	cb := legalRender(t, custom, handlers.LegalDisclaimerPage, nil).Body.String()
	if !strings.Contains(cb, `href="tel:999"`) || !strings.Contains(cb, `aria-label="Call 999"`) {
		t.Errorf("custom primary not used for the call link")
	}
	if !strings.Contains(legalText(t, cb), "Try 999 first.") {
		t.Errorf("older row note must end Try 999 first.")
	}
	if strings.Contains(cb, "sachet.ndma.gov.in") {
		t.Errorf("empty state lines must drop the SACHET link")
	}

	bare := handlers.AuthConfig{AssetVersion: "test", Chrome: handlers.Chrome{Numbers: handlers.EmergencyNumbers{Primary: "999"}}}
	bb := legalRender(t, bare, handlers.LegalDisclaimerPage, nil).Body.String()
	bt := legalText(t, bb)
	if strings.Contains(bt, "Try 999 first.") || strings.Contains(bt, "Older numbers") || strings.Contains(bt, "older number") {
		t.Errorf("empty older list must drop the second row")
	}
	if strings.Contains(bt, "state disaster helpline") || strings.Contains(bb, "sachet.ndma.gov.in") {
		t.Errorf("empty state lines must drop the third row and the SACHET link")
	}
}

func TestLegalPagesGlossaryUsesAppSentences(t *testing.T) {
	body := legalRender(t, handlers.AuthConfig{AssetVersion: "test"}, handlers.LegalDisclaimerPage, nil).Body.String()
	start := strings.Index(body, `<dl class="legal-glossary">`)
	if start < 0 {
		t.Fatalf("no legal-glossary list")
	}
	end := strings.Index(body[start:], "</dl>")
	if end < 0 {
		t.Fatalf("legal-glossary list never closes")
	}
	dl := body[start : start+end]

	var wantTerms, wantTexts []string
	for _, e := range service.DisclaimerGlossary() {
		wantTerms = append(wantTerms, e.Terms...)
		wantTexts = append(wantTexts, e.Text)
	}
	var gotTerms, gotTexts []string
	for _, m := range regexp.MustCompile(`(?s)<dt[^>]*>(.*?)</dt>`).FindAllStringSubmatch(dl, -1) {
		gotTerms = append(gotTerms, legalText(t, m[1]))
	}
	for _, m := range regexp.MustCompile(`(?s)<dd[^>]*>(.*?)</dd>`).FindAllStringSubmatch(dl, -1) {
		gotTexts = append(gotTexts, legalText(t, m[1]))
	}
	if !legalEqual(gotTerms, wantTerms) {
		t.Errorf("dt texts = %q, want %q", gotTerms, wantTerms)
	}
	if !legalEqual(gotTexts, wantTexts) {
		t.Errorf("dd texts = %q, want %q", gotTexts, wantTexts)
	}
	if len(gotTexts) < 2 {
		t.Fatalf("glossary has %d dd elements", len(gotTexts))
	}
	threshold := "Needs " + strconv.Itoa(service.IndependentAgreementThreshold) + " people at different spots to confirm it."
	if !strings.Contains(gotTexts[0], threshold) {
		t.Errorf("first dd %q lacks %q", gotTexts[0], threshold)
	}
	if gotTexts[1] != service.GlossaryCountsLead || strings.Contains(gotTexts[1], "Counts places") {
		t.Errorf("second dd = %q, want the counts lead alone", gotTexts[1])
	}

	text := legalText(t, body)
	if regexp.MustCompile(`(?i)confirmed by (\d|n\b)`).MatchString(text) {
		t.Errorf("page uses the older wording confirmed by N")
	}
	if service.ExpiryDuration(service.SeverityMedium) != service.ExpiryDuration(service.SeverityLow) {
		t.Fatalf("medium and low expiry differ, so Most reports is no longer true")
	}
	low := int(service.ExpiryDuration(service.SeverityLow).Hours())
	crit := int(service.ExpiryDuration(service.SeverityCritical).Hours())
	want := "Most reports last " + strconv.Itoa(low) + " hours and critical reports last " + strconv.Itoa(crit) + " hours."
	if !strings.Contains(text, want) {
		t.Errorf("page text lacks %q", want)
	}
	if strings.Contains(text, "unless re-confirmed") || strings.Contains(strings.ToLower(text), "fade") {
		t.Errorf("page uses the stale unless re-confirmed or fade wording")
	}
}

func TestLegalPagesPublicPrivateCache(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	newClient := func() *http.Client {
		jar, err := cookiejar.New(nil)
		if err != nil {
			t.Fatalf("cookie jar: %v", err)
		}
		return &http.Client{
			Jar:           jar,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	client := newClient()

	fetch := func(c *http.Client, path string) (*http.Response, string) {
		resp, err := c.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, string(b)
	}

	for _, path := range []string{"/disclaimer", "/policy"} {
		// A fresh visitor per page, so the first visit sets the cookie each time.
		resp, _ := fetch(newClient(), path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("logged out GET %s = %d, want 200", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Cache-Control"); got != "private, max-age=300" {
			t.Errorf("%s Cache-Control = %q", path, got)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s Content-Type = %q", path, ct)
		}
		var sawCookie bool
		for _, c := range resp.Cookies() {
			if c.Name == "pinalert_session" {
				sawCookie = true
			}
		}
		if !sawCookie {
			t.Errorf("%s did not set the pinalert_session cookie, so the private cache rule has no reason", path)
		}
	}

	verifySession(t, client, srv, mailer, "legal-reader@example.com")
	for _, path := range []string{"/disclaimer", "/policy"} {
		resp, body := fetch(client, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("logged in GET %s = %d, want 200", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Cache-Control"); got != "private, max-age=300" {
			t.Errorf("%s logged in Cache-Control = %q", path, got)
		}
		if strings.Contains(body, "legal-reader@example.com") || strings.Contains(body, "app-header") {
			t.Errorf("%s logged in body holds account data", path)
		}
	}
}
