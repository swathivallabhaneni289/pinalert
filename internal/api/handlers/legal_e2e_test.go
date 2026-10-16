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
	"time"

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

// Amended 2026-10-17 by Phase 3.4 plan 08 (03.4-CONTEXT D-31, D-06): the
// Disclaimer and the Policy are one reader, so /disclaimer and /policy both
// render the Disclaimer part and then the Policy part. The part headings
// Disclaimer and Policy open each part (the Policy one carries id policy so a
// link can land on it). The two pages differ only in the h1 and in which part
// link is the current one.
var legalDisclaimerH2s = []string{
	"Disclaimer",
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
	if n := strings.Count(body, `<h1 class="legal-reader__title">Disclaimer</h1>`); n != 1 {
		t.Errorf("h1 Disclaimer appears %d times, want 1", n)
	}
	if got := legalH2s(t, body); !legalEqual(got, legalAllH2s(false)) {
		t.Errorf("h2 sequence = %q, want %q", got, legalAllH2s(false))
	}
	for _, href := range []string{"/static/css/main.css?v=test", "/static/css/chrome.css?v=test", "/static/css/app-bar.css?v=test", "/static/js/theme.js?v=test"} {
		if n := strings.Count(body, href); n != 1 {
			t.Errorf("%s appears %d times, want 1", href, n)
		}
	}
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): reader.js is the second
	// script (the X goes back, /policy opens at the Policy part).
	if n := strings.Count(body, "<script"); n != 2 || strings.Count(body, "/static/js/reader.js?v=test") != 1 {
		t.Errorf("<script appears %d times, want 2 (theme.js and reader.js)", n)
	}
	// Amended 2026-10-16 by Phase 3.4 (owner decision, 03.4-CONTEXT D-29, D-30,
	// D-31): the bar on every page is replaced by the laptop strip on Nearby and
	// Post a report only. Phase 4 SC7 (both legal pages linked from the bar on
	// every page) is retired as worded; Disclaimer and Policy are reached from the
	// laptop strip's Read the disclaimer link, Settings, About Pinalert, the
	// first-time flow, the login screen and the one reader. The Phase 3.1 and
	// Phase 4 hands-on tests are walked on this layout after Phase 3.4 is built.
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the page carries no bar
	// and no strip. Its two part links (Disclaimer, Policy) are the only links
	// marked as current, and the X (a link named Close to "/") replaces the old
	// Back to Pinalert links.
	if strings.Contains(body, "disclaimer-bar") || strings.Contains(body, "notice-strip") {
		t.Errorf("the Disclaimer page carries no bar and no strip")
	}
	if n := strings.Count(body, `aria-current="page"`); n != 1 || !strings.Contains(body, `<a id="reader-part-disclaimer" href="#disclaimer" aria-current="page">Disclaimer</a>`) {
		t.Errorf("the Disclaimer page must mark exactly the Disclaimer part link as current")
	}
	if strings.Contains(body, "Back to Pinalert") {
		t.Errorf("the Back to Pinalert links are replaced by the X")
	}
	if n := strings.Count(body, `<a class="legal-close" id="reader-close-link" href="/" aria-label="Close"`); n != 1 {
		t.Errorf("the X link appears %d times, want 1", n)
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Errorf("page holds ZgotmplZ")
	}

	if n := strings.Count(body, "/static/css/legal.css?v=test"); n != 1 {
		t.Errorf("disclaimer legal.css appears %d times, want 1", n)
	}
	if !strings.Contains(body, `<a id="reader-part-policy" href="#policy">Policy</a>`) {
		t.Errorf("the Disclaimer page lacks the Policy part link")
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
	if n := strings.Count(pb, `<h1 class="legal-reader__title">Policy</h1>`); n != 1 {
		t.Errorf("h1 Policy appears %d times, want 1", n)
	}
	if got := legalH2s(t, pb); !legalEqual(got, legalAllH2s(false)) {
		t.Errorf("policy h2 sequence = %q, want %q", got, legalAllH2s(false))
	}
	ptext := legalText(t, pb)
	if !strings.Contains(ptext, "Rules, privacy and terms") {
		t.Errorf("policy subtitle missing")
	}
	for _, href := range []string{"/static/css/main.css?v=test", "/static/css/chrome.css?v=test", "/static/css/app-bar.css?v=test", "/static/css/legal.css?v=test", "/static/js/theme.js?v=test"} {
		if n := strings.Count(pb, href); n != 1 {
			t.Errorf("policy %s appears %d times, want 1", href, n)
		}
	}
	if n := strings.Count(pb, "<script"); n != 2 {
		t.Errorf("policy <script appears %d times, want 2 (theme.js and reader.js)", n)
	}
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the Policy part link is
	// the current one on /policy and the part heading carries id policy.
	if n := strings.Count(pb, `aria-current="page"`); n != 1 || !strings.Contains(pb, `<a id="reader-part-policy" href="#policy" aria-current="page">Policy</a>`) {
		t.Errorf("the Policy page must mark exactly the Policy part link as current")
	}
	if n := strings.Count(pb, `<h2 id="policy">Policy</h2>`); n != 1 {
		t.Errorf("the Policy heading with id policy appears %d times, want 1", n)
	}
	if strings.Contains(pb, "Back to Pinalert") || strings.Contains(pb, `class="legal-links"`) {
		t.Errorf("the Policy page keeps a Back to Pinalert link or the old footer; the X replaces them")
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
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): the Policy page also
	// holds the Disclaimer part, so its one tel link is the numbers block of
	// the Disclaimer. The Policy part itself still has none.
	if n := strings.Count(pb, "tel:"); n != 1 || strings.Contains(pb, "ZgotmplZ") {
		t.Errorf("policy page tel: occurrences = %d (want 1, in the Disclaimer part) or ZgotmplZ", n)
	}
	if _, policyPart, found := strings.Cut(pb, `<h2 id="policy">`); !found || strings.Contains(policyPart, "tel:") {
		t.Errorf("the Policy part must exist and hold no tel link")
	}
	for _, want := range []string{
		// D-24a, D-18: the Rescue needed category is retired, so the serious
		// report sentence names critical only and says what rescue reports
		// already posted keep.
		"If the check can't finish in time, a normal report waits and doesn't show to anyone until it does. A critical report is published at once and checked afterwards. A Rescue needed report posted before that category was retired keeps this rule until it fades.",
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
		// Quick 261016-pho: the place search now goes to Photon first, a
		// second third party, with the text and a position.
		"A typed place search goes through Pinalert to Photon, a search service run by komoot. Pinalert sends what you typed and the position of your pin or the middle of the map. If Photon can't answer, the search goes to OpenStreetMap's Nominatim service instead. The places in the results come from OpenStreetMap. Those services see the request the way any website does.",
	} {
		if !strings.Contains(ptext, want) {
			t.Errorf("policy text lacks %q", want)
		}
	}
	for _, banned := range []string{"threatening", "illegal", "critical or Rescue needed"} {
		if strings.Contains(strings.ToLower(ptext), strings.ToLower(banned)) {
			t.Errorf("policy text names %q", banned)
		}
	}
	// D-18, D-24a: the Disclaimer's expiry sentence names the places that exist.
	for _, want := range []string{
		"After a fixed time a report is marked Expired and leaves the Nearby list, whether or not anyone confirms it. Ended reports stay under My activity, Ended nearby, for 7 days. Most reports last",
	} {
		if !strings.Contains(ptext, want) {
			t.Errorf("page text lacks %q", want)
		}
	}
	for _, gone := range []string{"leaves the Active list", "stay visible for a while under Past"} {
		if strings.Contains(ptext, gone) {
			t.Errorf("page text still holds the outdated %q", gone)
		}
	}
}

// legalAllH2s is the h2 sequence of both legal pages: the Disclaimer part, then
// the Policy part. withContact adds the two contact headings.
func legalAllH2s(withContact bool) []string {
	out := append([]string{}, legalDisclaimerH2s...)
	if withContact {
		out = append(out, "Contact")
	}
	out = append(out, "Policy")
	policy := legalPolicyH2sWithoutContact
	if withContact {
		policy = append(append([]string{}, policy[:7]...), "Complaints and removal requests", "Terms", "Changes to this page")
	}
	return append(out, policy...)
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
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31): both pages hold both parts,
	// so the Contact heading closes the Disclaimer part and the Complaints
	// heading sits inside the Policy part.
	dh := legalH2s(t, d)
	if !legalEqual(dh, legalAllH2s(true)) {
		t.Errorf("disclaimer page h2 with contact = %q, want %q", dh, legalAllH2s(true))
	}
	if !strings.Contains(legalText(t, d), "To report abuse or ask for something to be removed, write to moderation@example.org.") {
		t.Errorf("disclaimer contact sentence missing")
	}
	if !strings.Contains(d, `href="mailto:moderation@example.org"`) {
		t.Errorf("disclaimer mailto link missing")
	}
	p := legalRender(t, set, handlers.LegalPolicyPage, nil).Body.String()
	if got := legalH2s(t, p); !legalEqual(got, legalAllH2s(true)) {
		t.Errorf("policy page h2 with contact = %q, want %q", got, legalAllH2s(true))
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
		"many states use 1070 for the state disaster helpline and 1077 for the district control room.",
		"These vary by state. Check your state disaster management authority's website.",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text lacks %q", want)
		}
	}
	if n := strings.Count(text, "national"); n != 1 {
		t.Errorf("the word national appears %d times, want 1", n)
	}
	// The SACHET help desk link was removed on 2026-10-16: a government check that
	// day found numbers on it that differ from the states' own sites.
	if strings.Contains(def, "sachet.ndma.gov.in") {
		t.Errorf("the SACHET help desk link must stay removed")
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
	// Amended 2026-10-17 by Phase 3.4 plan 08 (D-31, D-24a): the page now also
	// holds the Policy part, whose new sentence says a Rescue needed report
	// posted before the category was retired keeps its rule "until it fades".
	// The stale wording is still banned in the Disclaimer part, which is the
	// text before the Policy heading.
	disclaimerPart, _, _ := strings.Cut(body, `<h2 id="policy">`)
	if strings.Contains(text, "unless re-confirmed") || strings.Contains(strings.ToLower(legalText(t, disclaimerPart)), "fade") {
		t.Errorf("the Disclaimer part uses the stale unless re-confirmed or fade wording")
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

// TestReaderWindowRendersOnNearbyAndSettings proves D-31 end to end: Nearby and
// Settings carry the in app reader window, hidden, holding the very same legal
// text the public pages render, with the Policy anchor once, and no account
// data inside it (R-07, T-34-34, T-34-35).
func TestReaderWindowRendersOnNearbyAndSettings(t *testing.T) {
	srv, mailer, _ := newE2EServer(t)
	const email = "reader-window@example.com"
	client := newVerifiedClient(t, srv, mailer, email)

	public := legalText(t, legalRender(t, handlers.AuthConfig{AssetVersion: "test"}, handlers.LegalPolicyPage, nil).Body.String())
	// The box runs from its opening tag to the Last updated line, which closes
	// the Policy part.
	windowPattern := regexp.MustCompile(`(?s)<div class="app-window__box legal-text" id="reader-box" tabindex="0">(.*?<p class="legal-updated">.*?</p>)`)

	for _, path := range []string{"/", "/settings"} {
		resp, body := getPlacePage(t, client, srv.URL, path)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
		}
		if n := strings.Count(body, `id="reader-window" hidden`); n != 1 {
			t.Errorf("%s holds %d hidden reader windows, want 1", path, n)
		}
		if n := strings.Count(body, `id="policy"`); n != 1 {
			t.Errorf("%s holds id=policy %d times, want 1", path, n)
		}
		if n := strings.Count(body, "/static/js/reader.js?v="); n != 1 {
			t.Errorf("%s loads reader.js %d times, want 1", path, n)
		}
		inner := windowPattern.FindStringSubmatch(body)
		if inner == nil {
			t.Fatalf("%s: the reader box was not found", path)
		}
		if strings.Contains(inner[1], email) {
			t.Errorf("%s: the reader window holds the account email", path)
		}
		// The window text is the public text: the Disclaimer and Policy bodies
		// the standalone page renders (the Last updated line is in both).
		got := legalText(t, inner[1])
		if !strings.Contains(public, got) {
			t.Errorf("%s: the reader window text is not the text of the public pages", path)
		}
	}
}

// TestTermsVersionFollowsPolicyDate pins D-27: the terms version the first-time
// flow records is the day the Policy shows as last updated, so changing the text
// (and its date) asks everyone to agree again, and changing only one of the two
// fails here. The date is read from the rendered page because the constant that
// holds it is not exported.
func TestTermsVersionFollowsPolicyDate(t *testing.T) {
	rec := legalRender(t, handlers.AuthConfig{AssetVersion: "test"}, handlers.LegalPolicyPage, nil)
	updated := regexp.MustCompile(`(?s)<p class="legal-updated">Last updated (.*?)</p>`).FindStringSubmatch(rec.Body.String())
	if updated == nil {
		t.Fatalf("the Policy page lacks the Last updated line")
	}
	shown, err := time.Parse("2 January 2006", strings.TrimSpace(updated[1]))
	if err != nil {
		t.Fatalf("the Last updated date %q does not parse: %v", updated[1], err)
	}
	version, err := time.Parse("2006-01-02", service.CurrentTermsVersion)
	if err != nil {
		t.Fatalf("CurrentTermsVersion %q is not a YYYY-MM-DD date: %v", service.CurrentTermsVersion, err)
	}
	if !shown.Equal(version) {
		t.Errorf("the Policy says last updated %s but CurrentTermsVersion is %s; change them together (D-27)",
			shown.Format("2006-01-02"), service.CurrentTermsVersion)
	}
}
