package official

// These tests run the refresher against a real database and a fake SACHET: an
// httptest server that serves the recorded fixtures through the client's test
// seam. No test here reaches the real SACHET host. Each takes its clock from a
// fixed value, so expiry is decided by the clock and not by the wall.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pinalert/internal/areas"
	sqlcgen "pinalert/internal/store/sqlc"
	"pinalert/internal/testutil"
)

// dbNow is the instant the database tests run at: 15:00 in India on the day of
// the recorded messages. The Chennai, Andhra Pradesh and CWC messages are in
// force then; the Maharashtra update had ended.
var dbNow = time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)

const (
	guidChennai = "1792915039196007"
	guidAP      = "1792919196941008"
	guidCWC     = "1792897745415016"
	guidMah     = "1792859773702029"
	guidExc     = "1792000000000001"
)

type fakeItem struct {
	guid   string
	pub    time.Time
	issuer string
}

// fakeSachetServer is the upstream: an RSS list, CAP messages by guid, an ETag
// and a mode that makes it fail in one of the ways a real one does.
type fakeSachetServer struct {
	srv *httptest.Server

	mu                sync.Mutex
	items             []fakeItem
	caps              map[string][]byte
	etag              string
	ignoreConditional bool
	mode              string // "", status, timeout, garbage, oversize, empty, reset
	rssHits           int
	capHits           map[string]int
}

func newFakeSachetServer(t *testing.T) *fakeSachetServer {
	t.Helper()
	f := &fakeSachetServer{caps: map[string][]byte{}, capHits: map[string]int{}}
	mux := http.NewServeMux()
	mux.HandleFunc(sachetRSSPath, f.serveRSS)
	mux.HandleFunc(sachetCAPPath, f.serveCAP)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSachetServer) set(items []fakeItem, mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items, f.mode = items, mode
}

func (f *fakeSachetServer) setMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
}

func (f *fakeSachetServer) setCAP(guid string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.caps[guid] = body
}

func (f *fakeSachetServer) totalCAPHits() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, h := range f.capHits {
		n += h
	}
	return n
}

func (f *fakeSachetServer) serveRSS(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.rssHits++
	mode, items, etag, ignore := f.mode, append([]fakeItem(nil), f.items...), f.etag, f.ignoreConditional
	f.mu.Unlock()

	switch mode {
	case "status":
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	case "timeout":
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
		return
	case "reset":
		panic(http.ErrAbortHandler)
	case "garbage":
		w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
		_, _ = w.Write([]byte("<<< this is not a feed"))
		return
	case "oversize":
		w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
		_, _ = w.Write(bytes.Repeat([]byte(" "), RSSMaxBytes+16))
		return
	case "empty":
		items = nil
	}
	if etag != "" {
		w.Header().Set("ETag", etag)
		if !ignore && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>t</title>`)
	for _, it := range items {
		issuer := it.issuer
		if issuer == "" {
			issuer = "IMD Chennai"
		}
		fmt.Fprintf(&b, `<item><title>x</title><link>https://example.invalid/x</link><author>controlroom@ndma.gov.in (%s)</author><guid isPermaLink="false">%s</guid><pubDate>%s GMT</pubDate></item>`,
			issuer, it.guid, it.pub.UTC().Format("Mon, 02 Jan 2006 15:04:05"))
	}
	b.WriteString(`</channel></rss>`)
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	_, _ = w.Write([]byte(b.String()))
}

func (f *fakeSachetServer) serveCAP(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("identifier")
	f.mu.Lock()
	f.capHits[guid]++
	body, ok := f.caps[guid]
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/xml;charset=UTF-8")
	_, _ = w.Write(body)
}

// officialEnv is a refresher over the test database and the fake upstream.
type officialEnv struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
	srv  *fakeSachetServer
	clk  *clock
	r    *Refresher
}

func newOfficialEnv(t *testing.T, opts ...RefresherOption) *officialEnv {
	t.Helper()
	pool := testutil.NewTestDB(t)
	srv := newFakeSachetServer(t)
	clk := &clock{t: dbNow}
	q := sqlcgen.New(pool)
	all := append([]RefresherOption{
		WithRefresherClock(clk.now),
		WithSachet(testClient(srv.srv.URL, ClientConfig{}), areas.Default()),
	}, opts...)
	return &officialEnv{pool: pool, q: q, srv: srv, clk: clk, r: NewRefresher(pool, q, all...)}
}

// refresh moves the clock past the minimum gap and runs one refresh.
func (e *officialEnv) refresh(t *testing.T) (Result, error) {
	t.Helper()
	e.clk.set(e.clk.now().Add(MinRefreshGap))
	return e.r.RefreshIfDue(context.Background(), FeedSachet)
}

func (e *officialEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}

// inForce returns the guids in force for a reader in the given district and state.
func (e *officialEnv) inForce(t *testing.T, district, state int32) map[string]sqlcgen.OfficialAlertsInForceRow {
	t.Helper()
	rows, err := e.q.OfficialAlertsInForce(context.Background(), sqlcgen.OfficialAlertsInForceParams{
		Now: e.clk.now(), DistrictCodes: []int32{district}, StateCodes: []int32{state}})
	if err != nil {
		t.Fatalf("OfficialAlertsInForce: %v", err)
	}
	out := map[string]sqlcgen.OfficialAlertsInForceRow{}
	for _, r := range rows {
		out[r.Guid] = r
	}
	return out
}

func (e *officialEnv) alert(t *testing.T, guid string) sqlcgen.OfficialAlert {
	t.Helper()
	row, err := e.q.OfficialAlertForUpdate(context.Background(), guid) // a plain read: no transaction, the lock is released at once
	if err != nil {
		t.Fatalf("reading alert %s: %v", guid, err)
	}
	return row
}

func pub(minutes int) time.Time { return dbNow.Add(-time.Hour + time.Duration(minutes)*time.Minute) }

func capFixture(t *testing.T, name string) []byte { return fixture(t, name) }

func replaceOnce(t *testing.T, body []byte, old, repl string) []byte {
	t.Helper()
	if !bytes.Contains(body, []byte(old)) {
		t.Fatalf("fixture does not contain %q", old)
	}
	return bytes.ReplaceAll(body, []byte(old), []byte(repl))
}

// serveHeadFixtures sets up the five item feed of the first test.
func serveHeadFixtures(t *testing.T, srv *fakeSachetServer) {
	t.Helper()
	srv.set([]fakeItem{
		{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		{guid: guidAP, pub: pub(2), issuer: "Andhra Pradesh SDMA"},
		{guid: guidCWC, pub: pub(3), issuer: "CWC"},
		{guid: guidMah, pub: pub(4), issuer: "Maharashtra SDMA"},
		{guid: guidExc, pub: pub(5), issuer: "Test"},
	}, "")
	srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
	srv.setCAP(guidAP, capFixture(t, "cap_ap_sdma_lightning.xml"))
	srv.setCAP(guidCWC, capFixture(t, "cap_cwc_flood.xml"))
	srv.setCAP(guidMah, capFixture(t, "cap_maharashtra_update.xml"))
	srv.setCAP(guidExc, capFixture(t, "cap_exercise_synthetic.xml"))
}

func TestRefreshStoresAndPlaces(t *testing.T) {
	e := newOfficialEnv(t)
	serveHeadFixtures(t, e.srv)

	res, err := e.refresh(t)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if res.ItemsSeen != 5 || res.Fetched != 5 || res.Stored != 4 || res.Cancelled != 0 {
		t.Fatalf("result = %+v, want 5 seen, 5 fetched, 4 stored", res)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alerts`); n != 4 {
		t.Fatalf("%d alerts stored, want 4 (the exercise message is not kept)", n)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alerts WHERE guid = $1`, guidExc); n != 0 {
		t.Errorf("the exercise message was stored")
	}

	ix := areas.Default()
	tn, _ := ix.StateByName("Tamil Nadu")
	ap, _ := ix.StateByName("Andhra Pradesh")
	bihar, _ := ix.StateByName("Bihar")

	chennai := e.alert(t, guidChennai)
	if chennai.Issuer != "IMD Chennai" || chennai.Sender != "IMD-Chennai" || chennai.MsgType != "Alert" {
		t.Errorf("Chennai row = issuer %q sender %q type %q", chennai.Issuer, chennai.Sender, chennai.MsgType)
	}
	if !chennai.SentAt.Equal(time.Date(2026, 10, 25, 7, 56, 49, 0, time.UTC)) || !chennai.ExpiresAt.Equal(time.Date(2026, 10, 25, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("Chennai sent %v expires %v", chennai.SentAt, chennai.ExpiresAt)
	}
	if chennai.PlacementTier != "district" || chennai.ExpiresInferred {
		t.Errorf("Chennai tier %q inferred %v", chennai.PlacementTier, chennai.ExpiresInferred)
	}
	if !chennai.SourcePubdate.Valid || !chennai.SourcePubdate.Time.Equal(pub(1)) {
		t.Errorf("Chennai source_pubdate = %v, want %v", chennai.SourcePubdate, pub(1))
	}
	if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1 AND district_lgd IS NOT NULL`, chennai.ID); n != 23 {
		t.Errorf("Chennai has %d district rows, want 23", n)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1 AND state_lgd = $2`, chennai.ID, tn.Code); n != 1 {
		t.Errorf("Chennai has %d Tamil Nadu rows, want 1", n)
	}

	apRow := e.alert(t, guidAP)
	if apRow.PlacementTier != "state" || apRow.Issuer != "Andhra Pradesh SDMA" {
		t.Errorf("AP tier %q issuer %q, want state and Andhra Pradesh SDMA", apRow.PlacementTier, apRow.Issuer)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1`, apRow.ID); n != 1 {
		t.Errorf("AP has %d area rows, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1 AND state_lgd = $2 AND district_lgd IS NULL`, apRow.ID, ap.Code); n != 1 {
		t.Errorf("AP is not placed on Andhra Pradesh")
	}
	if cwc := e.alert(t, guidCWC); cwc.PlacementTier != "state" {
		t.Errorf("CWC tier %q, want state", cwc.PlacementTier)
	} else if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1 AND state_lgd = $2`, cwc.ID, bihar.Code); n != 1 {
		t.Errorf("CWC is not placed on Bihar")
	}

	st, err := e.r.Status(context.Background(), FeedSachet)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(e.clk.now()) || st.ItemsSeen != 5 || st.LastError != "" || st.ConsecutiveFailures != 0 {
		t.Errorf("status = %+v, want success now with 5 items", st)
	}
	// The other feed is untouched.
	if g, _ := e.r.Status(context.Background(), FeedGdacs); g.LastAttemptAt != nil {
		t.Errorf("a SACHET refresh touched the gdacs row: %+v", g)
	}

	// A reader in Chennai sees the Chennai alert and, by state, none of Bihar's.
	got := e.inForce(t, 568, int32(tn.Code))
	if _, ok := got[guidChennai]; !ok || len(got) != 1 {
		t.Errorf("in force for Chennai: %v, want only the Chennai alert", keys(got))
	}
	// The Maharashtra update ended before dbNow.
	if _, ok := e.inForce(t, 480, 27)[guidMah]; ok {
		t.Errorf("an ended alert is in force")
	}
}

func keys(m map[string]sqlcgen.OfficialAlertsInForceRow) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestRefreshUpdateAndCancel(t *testing.T) {
	e := newOfficialEnv(t)
	ix := areas.Default()
	tn, _ := ix.StateByName("Tamil Nadu")
	ap, _ := ix.StateByName("Andhra Pradesh")

	// 1. Two alerts: the Chennai warning and the Andhra Pradesh lightning alert.
	e.srv.set([]fakeItem{
		{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		{guid: guidAP, pub: pub(2), issuer: "Andhra Pradesh SDMA"},
	}, "")
	e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
	e.srv.setCAP(guidAP, capFixture(t, "cap_ap_sdma_lightning.xml"))
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if got := e.inForce(t, 568, int32(tn.Code)); len(got) != 1 || got[guidChennai].Guid == "" {
		t.Fatalf("first refresh: in force for Chennai %v", keys(got))
	}
	firstSeen := e.alert(t, guidAP).FirstSeenAt

	// 2. An Update under another guid names the Chennai alert in its references.
	update := replaceOnce(t, capFixture(t, "cap_maharashtra_update.xml"), "IN-1792859773702029_46", "IN-1792915039196007_43")
	e.srv.set([]fakeItem{
		{guid: guidMah, pub: pub(10), issuer: "Maharashtra SDMA"},
		{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		{guid: guidAP, pub: pub(2), issuer: "Andhra Pradesh SDMA"},
	}, "")
	e.srv.setCAP(guidMah, update)
	res, err := e.refresh(t)
	if err != nil || res.Fetched != 1 || res.Stored != 1 {
		t.Fatalf("update refresh: %+v, %v; want one fetched and stored", res, err)
	}
	chennai := e.alert(t, guidChennai)
	if !chennai.SupersededAt.Valid || chennai.CancelledAt.Valid {
		t.Errorf("Chennai superseded %v cancelled %v, want superseded only", chennai.SupersededAt, chennai.CancelledAt)
	}
	if got := e.inForce(t, 568, int32(tn.Code)); len(got) != 0 {
		t.Errorf("a superseded alert is still in force: %v", keys(got))
	}
	mah := e.alert(t, guidMah)
	if mah.MsgType != "Update" || mah.CapIdentifier != "IN-1792859773702029_29" || mah.ReferencesText != "IN-1792915039196007_43" {
		t.Errorf("update row = type %q id %q references %q", mah.MsgType, mah.CapIdentifier, mah.ReferencesText)
	}
	if mah.SupersededAt.Valid {
		t.Errorf("the update itself was marked superseded")
	}

	// 3. A new version of the AP series: same guid, newer pubDate.
	apNew := replaceOnce(t, capFixture(t, "cap_ap_sdma_lightning.xml"), "IN-1792919196941008_8", "IN-1792919196941008_9")
	e.srv.set([]fakeItem{
		{guid: guidMah, pub: pub(10), issuer: "Maharashtra SDMA"},
		{guid: guidAP, pub: pub(20), issuer: "Andhra Pradesh SDMA"},
		{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
	}, "")
	e.srv.setCAP(guidAP, apNew)
	res, err = e.refresh(t)
	if err != nil || res.Fetched != 1 {
		t.Fatalf("series refresh: %+v, %v; want one fetched", res, err)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alerts WHERE guid = $1`, guidAP); n != 1 {
		t.Fatalf("%d rows for the AP series, want 1", n)
	}
	apRow := e.alert(t, guidAP)
	if apRow.CapIdentifier != "IN-1792919196941008_9" || !apRow.FirstSeenAt.Equal(firstSeen) || !apRow.LastSeenAt.Equal(e.clk.now()) {
		t.Errorf("AP row = id %q first seen %v last seen %v; want _9, the first sighting kept, last seen now",
			apRow.CapIdentifier, apRow.FirstSeenAt, apRow.LastSeenAt)
	}
	if n := e.count(t, `SELECT count(*) FROM official_alert_areas WHERE alert_id = $1`, apRow.ID); n != 1 {
		t.Errorf("AP has %d area rows after the new version, want 1", n)
	}

	// 4. A Cancel under a new guid names the AP alert.
	cancel := replaceOnce(t, capFixture(t, "cap_cancel_synthetic.xml"), "IN-1792915039196007_43", "IN-1792919196941008_9")
	const guidCancel = "1792000000000099"
	e.srv.set([]fakeItem{
		{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
		{guid: guidMah, pub: pub(10), issuer: "Maharashtra SDMA"},
		{guid: guidAP, pub: pub(20), issuer: "Andhra Pradesh SDMA"},
	}, "")
	e.srv.setCAP(guidCancel, cancel)
	res, err = e.refresh(t)
	if err != nil || res.Cancelled != 1 || res.Stored != 0 {
		t.Fatalf("cancel refresh: %+v, %v; want one cancelled", res, err)
	}
	if got := e.inForce(t, 0, int32(ap.Code)); len(got) != 0 {
		t.Errorf("a cancelled alert is still in force: %v", keys(got))
	}
	apRow = e.alert(t, guidAP) // the row remains
	if !apRow.CancelledAt.Valid || !apRow.CancelledAt.Time.Equal(e.clk.now()) {
		t.Errorf("AP cancelled_at = %v, want now", apRow.CancelledAt)
	}
	if c := e.alert(t, guidCancel); !c.CancelledAt.Valid || c.MsgType != "Cancel" {
		t.Errorf("the cancel's own row = type %q cancelled %v, want a cancelled Cancel", c.MsgType, c.CancelledAt)
	}

	// 5. The same series is cancelled by its own guid (the shape of the fixture).
	cancelSame := capFixture(t, "cap_cancel_synthetic.xml")
	e.srv.set([]fakeItem{
		{guid: guidChennai, pub: pub(40), issuer: "IMD Chennai"},
		{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
	}, "")
	e.srv.setCAP(guidChennai, cancelSame)
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("same guid cancel: %v", err)
	}
	chennai = e.alert(t, guidChennai)
	if chennai.MsgType != "Cancel" || !chennai.CancelledAt.Valid || chennai.CapIdentifier != "IN-1792915039196007_44" {
		t.Errorf("Chennai after its own cancel = type %q cancelled %v id %q", chennai.MsgType, chennai.CancelledAt, chennai.CapIdentifier)
	}
	if got := e.inForce(t, 568, int32(tn.Code)); len(got) != 0 {
		t.Errorf("a cancelled series is in force: %v", keys(got))
	}

	// Absence from the feed never removes a row: the feed now lists none of them.
	e.srv.set([]fakeItem{{guid: "1792000000000050", pub: pub(50)}}, "")
	e.srv.setCAP("1792000000000050", capFixture(t, "cap_cwc_flood.xml"))
	before := e.count(t, `SELECT count(*) FROM official_alerts`)
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("absence refresh: %v", err)
	}
	if after := e.count(t, `SELECT count(*) FROM official_alerts`); after != before+1 {
		t.Errorf("%d rows after the feed dropped four, want %d: absence must not remove a row", after, before+1)
	}
}

func TestRefreshIsIdempotent(t *testing.T) {
	e := newOfficialEnv(t)
	e.srv.etag = `W/"stable"`
	serveHeadFixtures(t, e.srv)

	if _, err := e.refresh(t); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	alerts := e.count(t, `SELECT count(*) FROM official_alerts`)
	areaRows := e.count(t, `SELECT count(*) FROM official_alert_areas`)
	capHits := e.srv.totalCAPHits()
	if capHits != 5 {
		t.Fatalf("%d CAP requests on the first run, want 5", capHits)
	}

	// The second answer is a 304.
	res, err := e.refresh(t)
	if err != nil || !res.NotModified {
		t.Fatalf("second refresh: %+v, %v; want not modified", res, err)
	}
	// The third gets the full list again because the server ignores validators.
	e.srv.mu.Lock()
	e.srv.ignoreConditional = true
	e.srv.mu.Unlock()
	res, err = e.refresh(t)
	if err != nil || res.NotModified || res.ItemsSeen != 5 || res.Fetched != 0 {
		t.Fatalf("third refresh: %+v, %v; want 5 seen and nothing fetched", res, err)
	}

	if got := e.count(t, `SELECT count(*) FROM official_alerts`); got != alerts {
		t.Errorf("alerts %d -> %d over unchanged refreshes", alerts, got)
	}
	if got := e.count(t, `SELECT count(*) FROM official_alert_areas`); got != areaRows {
		t.Errorf("area rows %d -> %d over unchanged refreshes", areaRows, got)
	}
	if got := e.srv.totalCAPHits(); got != capHits {
		t.Errorf("%d CAP requests after three runs, want %d: nothing should be refetched", got, capHits)
	}
}

func TestRefreshFailuresKeepLastSuccess(t *testing.T) {
	e := newOfficialEnv(t)
	items := make([]fakeItem, 12)
	for i := range items {
		items[i] = fakeItem{guid: fmt.Sprintf("1792%012d", i+1), pub: pub(i)}
		e.srv.setCAP(items[i].guid, capFixture(t, "cap_ap_sdma_lightning.xml"))
	}
	e.srv.set(items, "")
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	good, err := e.r.Status(context.Background(), FeedSachet)
	if err != nil || good.LastSuccessAt == nil || good.ItemsSeen != 12 {
		t.Fatalf("status after the good run = %+v, %v", good, err)
	}

	steps := []struct{ mode, word string }{
		{"status", "status"},
		{"timeout", "timeout"},
		{"garbage", "parse"},
		{"oversize", "parse"},
		{"empty", "anomaly"},
		{"reset", "network"},
	}
	for i, step := range steps {
		e.srv.setMode(step.mode)
		res, err := e.refresh(t)
		if err == nil || res.ErrorClass != step.word {
			t.Fatalf("%s: got %+v, %v; want a %s failure", step.mode, res, err, step.word)
		}
		st, serr := e.r.Status(context.Background(), FeedSachet)
		if serr != nil {
			t.Fatalf("%s: Status: %v", step.mode, serr)
		}
		if st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(*good.LastSuccessAt) {
			t.Errorf("%s moved last_success_at: %v, want %v", step.mode, st.LastSuccessAt, good.LastSuccessAt)
		}
		if st.LastError != step.word || st.ConsecutiveFailures != i+1 {
			t.Errorf("%s: error %q failures %d, want %q and %d", step.mode, st.LastError, st.ConsecutiveFailures, step.word, i+1)
		}
		if st.ItemsSeen != 12 {
			t.Errorf("%s changed items_seen to %d", step.mode, st.ItemsSeen)
		}
		if n := e.count(t, `SELECT count(*) FROM official_alerts`); n != 12 {
			t.Errorf("%s: %d alerts stored, want 12 kept", step.mode, n)
		}
	}

	e.srv.setMode("")
	if res, err := e.refresh(t); err != nil || res.ErrorClass != "" {
		t.Fatalf("recovery: %+v, %v", res, err)
	}
	st, _ := e.r.Status(context.Background(), FeedSachet)
	if st.ConsecutiveFailures != 0 || st.LastError != "" || st.LastSuccessAt == nil || !st.LastSuccessAt.Equal(e.clk.now()) {
		t.Errorf("after recovery status = %+v, want a clean success now", st)
	}
}

func TestRefreshLeavesReportsAlone(t *testing.T) {
	e := newOfficialEnv(t)
	acct := testutil.SeedReporterAccount(t, e.pool, "official-bystander@example.com", "official-bystander")
	rep := testutil.SeedHistoryReport(t, e.pool, "official-bystander", "medium", "flood", dbNow.Add(time.Hour))
	testutil.MustExec(t, e.pool,
		`INSERT INTO votes (report_id, account_id, kind, value, geohash_cell) VALUES ($1, $2, 'content', 'confirm', 'tdr1qg0')`, rep, acct)

	tables := []string{"reports", "votes", "sessions", "accounts", "report_flags", "updates", "push_outbox"}
	counts := func() map[string]int {
		m := map[string]int{}
		for _, tb := range tables {
			m[tb] = e.count(t, "SELECT count(*) FROM "+tb)
		}
		return m
	}
	before := counts()
	if before["reports"] != 1 || before["votes"] != 1 {
		t.Fatalf("setup: %v", before)
	}

	serveHeadFixtures(t, e.srv)
	if _, err := e.refresh(t); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	for tb, n := range counts() {
		if n != before[tb] {
			t.Errorf("%s: %d rows before, %d after a refresh", tb, before[tb], n)
		}
	}
}

func TestRefreshColdStartOverRuns(t *testing.T) {
	e := newOfficialEnv(t)
	e.srv.etag = `W/"cold"`
	items := make([]fakeItem, 100)
	for i := range items {
		items[i] = fakeItem{guid: fmt.Sprintf("1792%012d", i+1), pub: pub(0).Add(time.Duration(i) * time.Second)}
		e.srv.setCAP(items[i].guid, capFixture(t, "cap_ap_sdma_lightning.xml"))
	}
	e.srv.set(items, "")

	for i, want := range []int{40, 40, 20} {
		before := e.srv.totalCAPHits()
		res, err := e.refresh(t)
		if err != nil || res.NotModified {
			t.Fatalf("run %d: %+v, %v", i+1, res, err)
		}
		if got := e.srv.totalCAPHits() - before; got != want || res.Fetched != want {
			t.Fatalf("run %d fetched %d (result says %d), want %d", i+1, got, res.Fetched, want)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM official_alerts`); n != 100 {
		t.Fatalf("%d alerts stored after three runs, want all 100", n)
	}
	// The feed is complete now, so the validators are stored and a fourth run is a 304.
	res, err := e.refresh(t)
	if err != nil || !res.NotModified {
		t.Fatalf("fourth run: %+v, %v; want not modified", res, err)
	}
	if got := e.srv.totalCAPHits(); got != 100 {
		t.Errorf("%d CAP requests in all, want exactly 100", got)
	}
}

func TestVersionHookInTransaction(t *testing.T) {
	var (
		mu      sync.Mutex
		fail    = true
		calls   []string
		seenIn  []bool   // whether the hook's own queries saw the row
		seenOut []string // the identifier another connection saw for the guid, or empty
		prevs   []bool
	)
	var env *officialEnv
	hook := func(ctx context.Context, q *sqlcgen.Queries, saved SavedVersion, prev *SavedVersion, now time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, saved.Version.GUID)
		prevs = append(prevs, prev != nil)
		_, inErr := q.OfficialAlertForUpdate(ctx, saved.Version.GUID)
		seenIn = append(seenIn, inErr == nil)
		var outside string
		if err := env.pool.QueryRow(ctx, `SELECT COALESCE(max(cap_identifier), '') FROM official_alerts WHERE guid = $1`, saved.Version.GUID).Scan(&outside); err != nil {
			return err
		}
		seenOut = append(seenOut, outside)
		if saved.ID == 0 || saved.Placement.Tier == "" {
			return errors.New("hook got an unfinished version")
		}
		if fail {
			return errors.New("hook refuses")
		}
		return nil
	}
	env = newOfficialEnv(t, WithVersionHook(hook))
	env.srv.set([]fakeItem{{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"}}, "")
	env.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))

	// A hook that fails rolls the version back: no alert row and no area rows.
	res, err := env.refresh(t)
	if err == nil {
		t.Fatalf("a failing hook did not fail the run: %+v", res)
	}
	if n := env.count(t, `SELECT count(*) FROM official_alerts`); n != 0 {
		t.Errorf("%d alert rows after a failed hook, want 0", n)
	}
	if n := env.count(t, `SELECT count(*) FROM official_alert_areas`); n != 0 {
		t.Errorf("%d area rows after a failed hook, want 0", n)
	}
	st, _ := env.r.Status(context.Background(), FeedSachet)
	if st.LastSuccessAt != nil {
		t.Errorf("a run whose only version failed moved last_success_at")
	}

	// The next run retries the version and, with a hook that succeeds, commits it.
	mu.Lock()
	fail = false
	mu.Unlock()
	res, err = env.refresh(t)
	if err != nil || res.Stored != 1 {
		t.Fatalf("retry: %+v, %v; want one stored", res, err)
	}
	if n := env.count(t, `SELECT count(*) FROM official_alerts`); n != 1 {
		t.Fatalf("%d alert rows after the retry, want 1", n)
	}
	if n := env.count(t, `SELECT count(*) FROM official_alert_areas WHERE district_lgd IS NOT NULL`); n != 23 {
		t.Errorf("%d district rows after the retry, want 23", n)
	}

	// A newer version of the same series reaches the hook with its predecessor.
	env.srv.set([]fakeItem{{guid: guidChennai, pub: pub(9), issuer: "IMD Chennai"}}, "")
	env.srv.setCAP(guidChennai, replaceOnce(t, capFixture(t, "cap_imd_chennai.xml"), "IN-1792915039196007_43", "IN-1792915039196007_45"))
	if _, err := env.refresh(t); err != nil {
		t.Fatalf("third run: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 {
		t.Fatalf("hook ran %d times, want 3: %v", len(calls), calls)
	}
	for i := range calls {
		if !seenIn[i] {
			t.Errorf("call %d: the hook's own queries did not see the row the transaction wrote", i+1)
		}
	}
	// Another connection sees only committed rows: nothing before the first commit,
	// and the old version while the new one is still in the transaction.
	if seenOut[0] != "" || seenOut[1] != "" || seenOut[2] != "IN-1792915039196007_43" {
		t.Errorf("another connection saw %q, want nothing, nothing and the old version _43 before the commit", seenOut)
	}
	if prevs[0] || prevs[1] || !prevs[2] {
		t.Errorf("prev given %v, want [false false true]: the first sighting has none, the new version has one", prevs)
	}
}

// TestRefreshWithdrawalStoredFirst pins CR-S01 of the Phase 5 review: a Cancel
// or an Update under another guid withdraws its target whatever the order the
// refresher stores them in (it stores the newest first), and a withdrawn target
// fetched again under its own guid with the same identifier stays withdrawn.
func TestRefreshWithdrawalStoredFirst(t *testing.T) {
	const guidCancel = "1792000000000099"
	const chennaiID = "IN-1792915039196007_43"

	withdrawn := func(t *testing.T, e *officialEnv, wantCancelled, wantSuperseded bool) {
		t.Helper()
		tn, _ := areas.Default().StateByName("Tamil Nadu")
		if got := e.inForce(t, 568, int32(tn.Code)); got[guidChennai].Guid != "" {
			t.Errorf("the withdrawn Chennai alert is in force: %v", keys(got))
		}
		row := e.alert(t, guidChennai)
		if row.CancelledAt.Valid != wantCancelled || row.SupersededAt.Valid != wantSuperseded {
			t.Errorf("Chennai cancelled %v superseded %v; want cancelled %t superseded %t",
				row.CancelledAt, row.SupersededAt, wantCancelled, wantSuperseded)
		}
	}

	t.Run("cancel stored before its target in one run", func(t *testing.T) {
		e := newOfficialEnv(t)
		e.srv.set([]fakeItem{
			{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
			{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		}, "")
		e.srv.setCAP(guidCancel, capFixture(t, "cap_cancel_synthetic.xml"))
		e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("refresh: %v", err)
		}
		withdrawn(t, e, true, false)
	})

	t.Run("cancel stored after its target", func(t *testing.T) {
		e := newOfficialEnv(t)
		e.srv.set([]fakeItem{{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"}}, "")
		e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("first refresh: %v", err)
		}
		e.srv.set([]fakeItem{
			{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
			{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		}, "")
		e.srv.setCAP(guidCancel, capFixture(t, "cap_cancel_synthetic.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("second refresh: %v", err)
		}
		withdrawn(t, e, true, false)
	})

	t.Run("update under a new guid stored a run before its target", func(t *testing.T) {
		e := newOfficialEnv(t)
		update := replaceOnce(t, capFixture(t, "cap_maharashtra_update.xml"), "IN-1792859773702029_46", chennaiID)
		items := []fakeItem{{guid: guidMah, pub: pub(55), issuer: "Maharashtra SDMA"}}
		flood := capFixture(t, "cap_cwc_flood.xml")
		for i := 0; i < CAPPerRun; i++ { // fillers between the update and its target
			g := fmt.Sprintf("17920000000%05d", 100+i)
			items = append(items, fakeItem{guid: g, pub: pub(50).Add(-time.Duration(i) * time.Second), issuer: "CWC"})
			e.srv.setCAP(g, flood)
		}
		items = append(items, fakeItem{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"})
		e.srv.set(items, "")
		e.srv.setCAP(guidMah, update)
		e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("first refresh: %v", err)
		}
		if n := e.count(t, `SELECT count(*) FROM official_alerts WHERE guid = $1`, guidChennai); n != 0 {
			t.Fatalf("the target was stored in the first run; the case needs it in the second")
		}
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("second refresh: %v", err)
		}
		withdrawn(t, e, false, true)
	})

	t.Run("update under a new guid stored after its target", func(t *testing.T) {
		e := newOfficialEnv(t)
		e.srv.set([]fakeItem{{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"}}, "")
		e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("first refresh: %v", err)
		}
		update := replaceOnce(t, capFixture(t, "cap_maharashtra_update.xml"), "IN-1792859773702029_46", chennaiID)
		e.srv.set([]fakeItem{
			{guid: guidMah, pub: pub(55), issuer: "Maharashtra SDMA"},
			{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		}, "")
		e.srv.setCAP(guidMah, update)
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("second refresh: %v", err)
		}
		withdrawn(t, e, false, true)
	})

	t.Run("withdrawn target fetched again under its own guid stays withdrawn", func(t *testing.T) {
		e := newOfficialEnv(t)
		e.srv.set([]fakeItem{{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"}}, "")
		e.srv.setCAP(guidChennai, capFixture(t, "cap_imd_chennai.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("first refresh: %v", err)
		}
		e.srv.set([]fakeItem{
			{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
			{guid: guidChennai, pub: pub(1), issuer: "IMD Chennai"},
		}, "")
		e.srv.setCAP(guidCancel, capFixture(t, "cap_cancel_synthetic.xml"))
		if _, err := e.refresh(t); err != nil {
			t.Fatalf("cancel refresh: %v", err)
		}
		// The same guid with a newer pubDate and the same identifier.
		e.srv.set([]fakeItem{
			{guid: guidCancel, pub: pub(30), issuer: "IMD Chennai"},
			{guid: guidChennai, pub: pub(40), issuer: "IMD Chennai"},
		}, "")
		res, err := e.refresh(t)
		if err != nil || res.Fetched != 1 {
			t.Fatalf("refetch refresh: %+v, %v; want the target fetched again", res, err)
		}
		withdrawn(t, e, true, false)
	})
}
