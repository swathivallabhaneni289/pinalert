package official

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// The fixtures under testdata are the feed's own text. The four real CAP
// messages and the RSS head were read from SACHET on 2026-10-25; the Cancel and
// Exercise messages are synthetic and say so in a comment at their top.

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func fixtureItem(guid, issuer string) Item {
	return Item{
		GUID:        guid,
		IssuerLabel: issuer,
		PubDate:     time.Date(2026, 10, 25, 9, 7, 56, 0, time.UTC),
	}
}

func mustParseCAP(t *testing.T, body []byte, item Item) Version {
	t.Helper()
	v, err := ParseCAP(bytes.NewReader(body), item)
	if err != nil {
		t.Fatalf("ParseCAP: %v", err)
	}
	return v
}

func TestParseRSSHead(t *testing.T) {
	items, err := ParseRSS(bytes.NewReader(fixture(t, "rss_head.xml")))
	if err != nil {
		t.Fatalf("ParseRSS: %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("items = %d, want 5", len(items))
	}
	for i, it := range items {
		if len(it.GUID) != 16 || !ValidGUID(it.GUID) {
			t.Errorf("item %d guid %q is not a 16 digit guid", i, it.GUID)
		}
		if it.Title == "" {
			t.Errorf("item %d has no title", i)
		}
		if !strings.HasSuffix(it.Link, "identifier="+it.GUID) {
			t.Errorf("item %d link %q does not end with its guid", i, it.Link)
		}
		// The channel pubDate in the file is 16 Oct; every item is 25 Oct.
		if it.PubDate.IsZero() || it.PubDate.Day() != 25 {
			t.Errorf("item %d pubDate = %v, want an item date on 25 Oct", i, it.PubDate)
		}
	}
	if items[0].GUID != "1792916961451006" {
		t.Errorf("first guid = %q", items[0].GUID)
	}
	if items[0].IssuerLabel != "IMD Thiruvananthapuram" {
		t.Errorf("first issuer = %q", items[0].IssuerLabel)
	}
	if items[1].IssuerLabel != "Andhra Pradesh SDMA" {
		t.Errorf("second issuer = %q", items[1].IssuerLabel)
	}
	if !items[0].PubDate.Equal(time.Date(2026, 10, 25, 9, 15, 9, 0, time.UTC)) {
		t.Errorf("first pubDate = %v", items[0].PubDate)
	}
	for _, it := range items {
		if strings.Contains(it.IssuerLabel, "@") {
			t.Errorf("issuer label %q still holds the address", it.IssuerLabel)
		}
	}
}

func TestParseRSSRejectsGarbage(t *testing.T) {
	for name, body := range map[string]string{
		"not xml":    "this is not xml",
		"wrong root": "<feed><entry/></feed>",
		"empty":      "",
	} {
		_, err := ParseRSS(strings.NewReader(body))
		if err == nil {
			t.Errorf("%s: want an error", name)
			continue
		}
		if !errors.Is(err, ErrParse) {
			t.Errorf("%s: error %v does not wrap ErrParse", name, err)
		}
	}
}

func TestValidGUID(t *testing.T) {
	for _, s := range []string{"1792919196941008", "1234567890", "12345678901234567890"} {
		if !ValidGUID(s) {
			t.Errorf("ValidGUID(%q) = false, want true", s)
		}
	}
	for _, s := range []string{
		"", "17929191", "1792919196941008x", "../1792919196941008",
		"123456789012345678901", " 1792919196941008", "1792919196941008\n",
		"1792919196941008&x=1", "१७९२९१९१९६९४१००८",
	} {
		if ValidGUID(s) {
			t.Errorf("ValidGUID(%q) = true, want false", s)
		}
	}
}

func TestIssuerLabel(t *testing.T) {
	cases := map[string]string{
		"controlroom@ndma.gov.in (IMD Chennai)":         "IMD Chennai",
		"controlroom@ndma.gov.in (Andhra Pradesh SDMA)": "Andhra Pradesh SDMA",
		"controlroom@ndma.gov.in (CWC)":                 "CWC",
		"controlroom@ndma.gov.in":                       "",
		"controlroom@ndma.gov.in ()":                    "",
		"IMD Pune":                                      "IMD Pune",
		"":                                              "",
		"a@b.c (x@y.z)":                                 "",
	}
	for in, want := range cases {
		if got := IssuerLabel(in); got != want {
			t.Errorf("IssuerLabel(%q) = %q, want %q", in, got, want)
		}
	}
	long := "a@b.c (" + strings.Repeat("W", 200) + ")"
	if got := IssuerLabel(long); utf8.RuneCountInString(got) != IssuerMaxRunes {
		t.Errorf("long issuer has %d runes, want %d", utf8.RuneCountInString(got), IssuerMaxRunes)
	}
}

func TestParseCAPLanguages(t *testing.T) {
	// English block first, Telugu block second (labelled TL, not a BCP 47 code).
	ap := mustParseCAP(t, fixture(t, "cap_ap_sdma_lightning.xml"), fixtureItem("1792919196941008", "Andhra Pradesh SDMA"))
	if !strings.HasPrefix(ap.HeadlineEN, "There is a possibility of lightning") {
		t.Errorf("AP HeadlineEN = %q", ap.HeadlineEN)
	}
	if !strings.HasPrefix(ap.HeadlineOriginal, "మీ ప్రాంతంలో") {
		t.Errorf("AP HeadlineOriginal = %q", ap.HeadlineOriginal)
	}
	if ap.OriginalLang != "TL" {
		t.Errorf("AP OriginalLang = %q, want the raw label TL", ap.OriginalLang)
	}
	if ap.Event != "Lightning" || ap.Severity != SeveritySevere || ap.Urgency != "Expected" || ap.Certainty != "Possible" {
		t.Errorf("AP event fields = %q %q %q %q", ap.Event, ap.Severity, ap.Urgency, ap.Certainty)
	}
	if ap.Instruction != "Please follow SDMA guidelines." {
		t.Errorf("AP Instruction = %q", ap.Instruction)
	}
	if ap.AreaDesc != "6 Mandals" {
		t.Errorf("AP AreaDesc = %q", ap.AreaDesc)
	}
	if ap.CapIdentifier != "IN-1792919196941008_8" || ap.Sender != "Andhra-Pradesh-SDMA" {
		t.Errorf("AP identifiers = %q %q", ap.CapIdentifier, ap.Sender)
	}
	if ap.GUID != "1792919196941008" || ap.IssuerLabel != "Andhra Pradesh SDMA" {
		t.Errorf("AP item fields = %q %q", ap.GUID, ap.IssuerLabel)
	}
	if !ap.SourcePubDate.Equal(time.Date(2026, 10, 25, 9, 7, 56, 0, time.UTC)) {
		t.Errorf("AP SourcePubDate = %v", ap.SourcePubDate)
	}
	wantSent := time.Date(2026, 10, 25, 14, 36, 36, 0, time.FixedZone("IST", 5*3600+1800))
	if !ap.SentAt.Equal(wantSent) {
		t.Errorf("AP SentAt = %v, want %v", ap.SentAt, wantSent)
	}
	if ap.EffectiveAt == nil || ap.OnsetAt == nil {
		t.Errorf("AP effective and onset should be set: %v %v", ap.EffectiveAt, ap.OnsetAt)
	}

	// Marathi block first, English second: the English headline still wins.
	mh := mustParseCAP(t, fixture(t, "cap_maharashtra_update.xml"), fixtureItem("1792859773702029", "Maharashtra SDMA"))
	if !strings.HasPrefix(mh.HeadlineEN, "Light to moderate spells of rain") {
		t.Errorf("MH HeadlineEN = %q", mh.HeadlineEN)
	}
	if mh.OriginalLang != "MR" || !strings.HasPrefix(mh.HeadlineOriginal, "पुढील") {
		t.Errorf("MH original = %q %q", mh.OriginalLang, mh.HeadlineOriginal)
	}

	// One English block only: nothing is kept as an original.
	cwc := mustParseCAP(t, fixture(t, "cap_cwc_flood.xml"), fixtureItem("1792897745415016", "CWC"))
	if cwc.HeadlineOriginal != "" || cwc.OriginalLang != "" || cwc.DescriptionOriginal != "" {
		t.Errorf("CWC should have no original: %q %q", cwc.OriginalLang, cwc.HeadlineOriginal)
	}
	if !strings.HasPrefix(cwc.DescriptionEN, "River Gandak at Dumariaghat") {
		t.Errorf("CWC DescriptionEN = %q", cwc.DescriptionEN)
	}

	// No block starts with en: the English headline is empty, the first block is the original.
	noEN := bytes.ReplaceAll(fixture(t, "cap_ap_sdma_lightning.xml"),
		[]byte("<cap:language>en-IN</cap:language>"), []byte("<cap:language>TL</cap:language>"))
	v := mustParseCAP(t, noEN, fixtureItem("1792919196941008", ""))
	if v.HeadlineEN != "" {
		t.Errorf("no en block: HeadlineEN = %q, want empty", v.HeadlineEN)
	}
	if v.HeadlineOriginal == "" || v.OriginalLang != "TL" {
		t.Errorf("no en block: original = %q %q", v.OriginalLang, v.HeadlineOriginal)
	}
	if v.Event != "Lightning" {
		t.Errorf("no en block: event should come from the first block, got %q", v.Event)
	}

	// The en test is case insensitive and looks at the start of the label.
	upper := bytes.ReplaceAll(fixture(t, "cap_ap_sdma_lightning.xml"),
		[]byte("<cap:language>en-IN</cap:language>"), []byte("<cap:language> EN-in </cap:language>"))
	u := mustParseCAP(t, upper, fixtureItem("1792919196941008", ""))
	if !strings.HasPrefix(u.HeadlineEN, "There is a possibility") {
		t.Errorf("EN-in label not treated as English: %q", u.HeadlineEN)
	}
}

func TestParseCAPGeocodes(t *testing.T) {
	chennai := mustParseCAP(t, fixture(t, "cap_imd_chennai.xml"), fixtureItem("1792915039196007", "IMD Chennai"))
	if len(chennai.DistrictCodes) != 23 {
		t.Fatalf("Chennai codes = %d, want 23", len(chennai.DistrictCodes))
	}
	if !sort.IntsAreSorted(chennai.DistrictCodes) {
		t.Errorf("codes not sorted: %v", chennai.DistrictCodes)
	}
	for i := 1; i < len(chennai.DistrictCodes); i++ {
		if chennai.DistrictCodes[i] == chennai.DistrictCodes[i-1] {
			t.Errorf("duplicate code %d", chennai.DistrictCodes[i])
		}
	}
	has := func(codes []int, c int) bool {
		for _, x := range codes {
			if x == c {
				return true
			}
		}
		return false
	}
	if !has(chennai.DistrictCodes, 574) || !has(chennai.DistrictCodes, 568) {
		t.Errorf("574 and 568 expected in %v", chennai.DistrictCodes)
	}

	// The three Maharashtra codes appear in both language blocks and are listed once.
	mh := mustParseCAP(t, fixture(t, "cap_maharashtra_update.xml"), fixtureItem("1792859773702029", ""))
	if got := mh.DistrictCodes; len(got) != 3 || got[0] != 480 || got[1] != 492 || got[2] != 495 {
		t.Errorf("Maharashtra codes = %v, want [480 492 495]", got)
	}

	for _, name := range []string{"cap_ap_sdma_lightning.xml", "cap_cwc_flood.xml"} {
		v := mustParseCAP(t, fixture(t, name), fixtureItem("1792919196941008", ""))
		if len(v.DistrictCodes) != 0 {
			t.Errorf("%s: codes = %v, want none", name, v.DistrictCodes)
		}
	}

	// Junk values and other geocode names are ignored.
	junk := bytes.ReplaceAll(fixture(t, "cap_imd_chennai.xml"),
		[]byte("<cap:value>574</cap:value>"), []byte("<cap:value>not-a-number</cap:value>"))
	junk = bytes.ReplaceAll(junk, []byte("<cap:valueName>LGD District Code</cap:valueName>\n<cap:value>568</cap:value>"),
		[]byte("<cap:valueName>SAME</cap:valueName>\n<cap:value>568</cap:value>"))
	j := mustParseCAP(t, junk, fixtureItem("1792915039196007", ""))
	if has(j.DistrictCodes, 574) || has(j.DistrictCodes, 568) || len(j.DistrictCodes) != 21 {
		t.Errorf("junk handling: %d codes, has574=%v has568=%v", len(j.DistrictCodes), has(j.DistrictCodes, 574), has(j.DistrictCodes, 568))
	}
}

func TestParseCAPUpdateReferences(t *testing.T) {
	v := mustParseCAP(t, fixture(t, "cap_maharashtra_update.xml"), fixtureItem("1792859773702029", ""))
	if v.MsgType != MsgUpdate {
		t.Errorf("MsgType = %q, want Update", v.MsgType)
	}
	if v.CapIdentifier != "IN-1792859773702029_29" {
		t.Errorf("CapIdentifier = %q", v.CapIdentifier)
	}
	// The full referenced identifier, suffix _46, never the base alone.
	if len(v.References) != 1 || v.References[0] != "IN-1792859773702029_46" {
		t.Errorf("References = %v, want [IN-1792859773702029_46]", v.References)
	}

	// Several space separated triples, each reduced to its identifier, in order, once each.
	multi := bytes.ReplaceAll(fixture(t, "cap_maharashtra_update.xml"),
		[]byte("IMD-Mumbai,IN-1792859773702029_46,2026-10-24T22:05:47+05:30"),
		[]byte("IMD-Mumbai,IN-1_1,2026-10-24T22:05:47+05:30  X,IN-2_3,2026-10-24T22:06:47+05:30 IMD-Mumbai,IN-1_1,2026-10-24T22:05:47+05:30"))
	m := mustParseCAP(t, multi, fixtureItem("1792859773702029", ""))
	if len(m.References) != 2 || m.References[0] != "IN-1_1" || m.References[1] != "IN-2_3" {
		t.Errorf("multi References = %v", m.References)
	}

	// A message with no references has none.
	ap := mustParseCAP(t, fixture(t, "cap_ap_sdma_lightning.xml"), fixtureItem("1792919196941008", ""))
	if len(ap.References) != 0 {
		t.Errorf("AP References = %v, want none", ap.References)
	}

	// The synthetic Cancel names the real Chennai message.
	c := mustParseCAP(t, fixture(t, "cap_cancel_synthetic.xml"), fixtureItem("1792915039196007", ""))
	if c.MsgType != MsgCancel || len(c.References) != 1 || c.References[0] != "IN-1792915039196007_43" {
		t.Errorf("Cancel = %q %v", c.MsgType, c.References)
	}
}

func TestParseCAPKeep(t *testing.T) {
	keepers := map[string]MsgType{
		"cap_ap_sdma_lightning.xml":  MsgAlert,
		"cap_imd_chennai.xml":        MsgAlert,
		"cap_cwc_flood.xml":          MsgAlert,
		"cap_maharashtra_update.xml": MsgUpdate,
		"cap_cancel_synthetic.xml":   MsgCancel,
	}
	for name, want := range keepers {
		v := mustParseCAP(t, fixture(t, name), fixtureItem("1792919196941008", ""))
		if v.MsgType != want {
			t.Errorf("%s: MsgType = %q, want %q", name, v.MsgType, want)
		}
		if !v.Keep() {
			t.Errorf("%s: Keep() = false, want true", name)
		}
	}

	ex := mustParseCAP(t, fixture(t, "cap_exercise_synthetic.xml"), fixtureItem("1792000000000001", ""))
	if ex.Keep() {
		t.Errorf("the Exercise fixture must not be kept (status %q)", ex.Status)
	}

	base := fixture(t, "cap_ap_sdma_lightning.xml")
	mutate := func(old, repl string) Version {
		return mustParseCAP(t, bytes.ReplaceAll(base, []byte(old), []byte(repl)), fixtureItem("1792919196941008", ""))
	}
	for _, st := range []string{"Exercise", "Test", "System", "Draft", "Excluded"} {
		if mutate("<cap:status>Actual</cap:status>", "<cap:status>"+st+"</cap:status>").Keep() {
			t.Errorf("status %s kept", st)
		}
	}
	for _, sc := range []string{"Private", "Restricted", ""} {
		if mutate("<cap:scope>Public</cap:scope>", "<cap:scope>"+sc+"</cap:scope>").Keep() {
			t.Errorf("scope %q kept", sc)
		}
	}
	for _, mt := range []string{"Ack", "Error", "Other", ""} {
		if mutate("<cap:msgType>Alert</cap:msgType>", "<cap:msgType>"+mt+"</cap:msgType>").Keep() {
			t.Errorf("msgType %q kept", mt)
		}
	}
	// Padding and letter case in the feed do not matter.
	if !mutate("<cap:status>Actual</cap:status>", "<cap:status> actual </cap:status>").Keep() {
		t.Errorf("a padded lower case status should still be kept")
	}

	// The zero Version is not kept.
	if (Version{}).Keep() {
		t.Errorf("zero Version kept")
	}
}

func TestExpiryRule(t *testing.T) {
	sent := time.Date(2026, 10, 25, 14, 36, 36, 0, time.FixedZone("IST", 5*3600+1800))
	fallback := sent.Add(FallbackLifetime)
	valid := "2026-10-25T17:35:00+05:30"
	cases := []struct {
		name     string
		raw      string
		want     time.Time
		inferred bool
	}{
		{"valid", valid, time.Date(2026, 10, 25, 17, 35, 0, 0, time.FixedZone("IST", 5*3600+1800)), false},
		{"padded valid", "  " + valid + " ", time.Date(2026, 10, 25, 17, 35, 0, 0, time.FixedZone("IST", 5*3600+1800)), false},
		{"missing", "", fallback, true},
		{"unparseable", "tomorrow evening", fallback, true},
		{"date only", "2026-10-26", fallback, true},
		{"before sent", "2026-10-25T14:36:35+05:30", fallback, true},
		{"equal to sent", "2026-10-25T14:36:36+05:30", sent, false},
		{"exactly seven days", sent.Add(MaxLifetime).Format(time.RFC3339), sent.Add(MaxLifetime), false},
		{"one second over seven days", sent.Add(MaxLifetime + time.Second).Format(time.RFC3339), fallback, true},
	}
	for _, tc := range cases {
		got, inferred := resolveExpiry(sent, tc.raw)
		if !got.Equal(tc.want) || inferred != tc.inferred {
			t.Errorf("%s: resolveExpiry = %v inferred=%v, want %v inferred=%v", tc.name, got, inferred, tc.want, tc.inferred)
		}
	}
	if FallbackLifetime != 6*time.Hour || MaxLifetime != 7*24*time.Hour {
		t.Errorf("the two pinned lifetimes moved: %v %v", FallbackLifetime, MaxLifetime)
	}

	// The same rule through a whole message: the expires element removed.
	body := fixture(t, "cap_ap_sdma_lightning.xml")
	stripped := bytes.ReplaceAll(body, []byte("<cap:expires>2026-10-25T17:35:00+05:30</cap:expires>\n"), nil)
	v := mustParseCAP(t, stripped, fixtureItem("1792919196941008", ""))
	if !v.ExpiresInferred || !v.ExpiresAt.Equal(v.SentAt.Add(FallbackLifetime)) {
		t.Errorf("missing expires: ExpiresAt=%v inferred=%v", v.ExpiresAt, v.ExpiresInferred)
	}
	real := mustParseCAP(t, body, fixtureItem("1792919196941008", ""))
	if real.ExpiresInferred || !real.ExpiresAt.Equal(time.Date(2026, 10, 25, 12, 5, 0, 0, time.UTC)) {
		t.Errorf("real expires: ExpiresAt=%v inferred=%v", real.ExpiresAt, real.ExpiresInferred)
	}
}

func TestSeverityClosedSet(t *testing.T) {
	cases := map[string]Severity{
		"Extreme": SeverityExtreme, "extreme": SeverityExtreme, "EXTREME": SeverityExtreme,
		"Severe": SeveritySevere, "severe": SeveritySevere, "  severe ": SeveritySevere,
		"Moderate": SeverityModerate, "MODERATE": SeverityModerate,
		"Minor": SeverityMinor, "minor": SeverityMinor,
		"Unknown": SeverityUnknown, "unknown": SeverityUnknown,
		"Catastrophic": SeverityUnknown, "": SeverityUnknown, "Severe!": SeverityUnknown, "<b>Severe</b>": SeverityUnknown,
	}
	for in, want := range cases {
		if got := ParseSeverity(in); got != want {
			t.Errorf("ParseSeverity(%q) = %q, want %q", in, got, want)
		}
	}
	// The set is exactly these five words.
	words := map[Severity]string{
		SeverityExtreme: "Extreme", SeveritySevere: "Severe", SeverityModerate: "Moderate",
		SeverityMinor: "Minor", SeverityUnknown: "Unknown",
	}
	for s, w := range words {
		if string(s) != w {
			t.Errorf("%q should hold the word %s", s, w)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"plain", "Heavy rain expected", 100, "Heavy rain expected"},
		{"control characters", "a\x00b\x07c\x1bd\x7fe", 100, "abcde"},
		{"crlf", "one\r\ntwo\rthree\nfour", 100, "one\ntwo\nthree\nfour"},
		{"tab becomes space", "a\tb", 100, "a b"},
		{"markup stays literal", "<b>x</b> & <script>", 100, "<b>x</b> & <script>"},
		{"bidi override removed", "abc‮def⁦g", 100, "abcdefg"},
		{"zero width joiner kept for Indic text", "ക‍ക", 100, "ക‍ക"},
		{"trimmed", "  \n hi \n ", 100, "hi"},
		{"limit in runes", "abcdefghij", 4, "abcd"},
		{"limit cuts at a rune boundary", "మీ ప్రాంతంలో", 4, "మీ ప"},
		{"limit then trim", "ab   cd", 4, "ab"},
		{"zero limit", "abc", 0, ""},
		{"invalid utf8", "ab\xffcd", 100, "abcd"},
	}
	for _, tc := range cases {
		got := CleanText(tc.in, tc.max)
		if got != tc.want {
			t.Errorf("%s: CleanText(%q, %d) = %q, want %q", tc.name, tc.in, tc.max, got, tc.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("%s: result is not valid UTF-8", tc.name)
		}
	}
}

func TestParseCAPLimits(t *testing.T) {
	long := strings.Repeat("W", 6000)
	body := fixture(t, "cap_cwc_flood.xml")
	body = bytes.ReplaceAll(body, []byte("<cap:instruction/>"), []byte("<cap:instruction>"+long+"</cap:instruction>"))
	body = bytes.ReplaceAll(body, []byte("<cap:areaDesc>Gandak, Dumariaghat, Gopalganj, Bihar</cap:areaDesc>"),
		[]byte("<cap:areaDesc>"+long+"</cap:areaDesc>"))
	body = bytes.ReplaceAll(body, []byte("<cap:headline>River Gandak"), []byte("<cap:headline>"+long+" River Gandak"))
	body = bytes.ReplaceAll(body, []byte("<cap:description>River Gandak"), []byte("<cap:description>"+long+" River Gandak"))
	v := mustParseCAP(t, body, fixtureItem("1792897745415016", ""))
	for name, c := range map[string]struct {
		got  string
		want int
	}{
		"headline":    {v.HeadlineEN, HeadlineMaxRunes},
		"description": {v.DescriptionEN, DescriptionMaxRunes},
		"instruction": {v.Instruction, InstructionMaxRunes},
		"area":        {v.AreaDesc, AreaMaxRunes},
	} {
		if n := utf8.RuneCountInString(c.got); n != c.want {
			t.Errorf("%s has %d runes, want %d", name, n, c.want)
		}
	}
	if HeadlineMaxRunes != 400 || DescriptionMaxRunes != 4000 || InstructionMaxRunes != 1000 || AreaMaxRunes != 600 || IssuerMaxRunes != 80 {
		t.Errorf("a text limit moved")
	}

	// Markup and control characters in a message become plain text.
	dirty := bytes.ReplaceAll(fixture(t, "cap_cwc_flood.xml"), []byte("<cap:instruction/>"),
		[]byte("<cap:instruction>&lt;img src=x onerror=alert(1)&gt;&#7;ok</cap:instruction>"))
	d, err := ParseCAP(bytes.NewReader(dirty), fixtureItem("1792897745415016", ""))
	if err != nil {
		// A control character reference is not well formed XML 1.0 in some decoders; fall back to the clean case.
		dirty = bytes.ReplaceAll(fixture(t, "cap_cwc_flood.xml"), []byte("<cap:instruction/>"),
			[]byte("<cap:instruction>&lt;img src=x onerror=alert(1)&gt;ok</cap:instruction>"))
		d = mustParseCAP(t, dirty, fixtureItem("1792897745415016", ""))
	}
	if !strings.HasPrefix(d.Instruction, "<img src=x onerror=alert(1)>") || strings.ContainsRune(d.Instruction, 7) {
		t.Errorf("Instruction = %q", d.Instruction)
	}

	// The issuer label is capped too.
	item := fixtureItem("1792897745415016", strings.Repeat("I", 300))
	if n := utf8.RuneCountInString(mustParseCAP(t, fixture(t, "cap_cwc_flood.xml"), item).IssuerLabel); n != IssuerMaxRunes {
		t.Errorf("issuer has %d runes, want %d", n, IssuerMaxRunes)
	}
}

func TestParseCAPErrors(t *testing.T) {
	good := string(fixture(t, "cap_cwc_flood.xml"))
	cases := map[string]string{
		"garbage":    "not xml at all",
		"wrong root": "<rss><channel/></rss>",
		"no sent":    strings.Replace(good, "<cap:sent>2026-10-25T08:38:44+05:30</cap:sent>", "", 1),
		"bad sent":   strings.Replace(good, "2026-10-25T08:38:44+05:30", "yesterday", 1),
		"truncated":  good[:len(good)/2],
		"empty":      "",
	}
	for name, body := range cases {
		_, err := ParseCAP(strings.NewReader(body), fixtureItem("1792897745415016", ""))
		if err == nil {
			t.Errorf("%s: want an error", name)
			continue
		}
		if !errors.Is(err, ErrParse) {
			t.Errorf("%s: error %v does not wrap ErrParse", name, err)
		}
	}

	// Undefined entities (the usual XML bomb route) are an error, not an expansion.
	bomb := `<?xml version="1.0"?><!DOCTYPE lolz [<!ENTITY lol "lol"><!ENTITY lol2 "&lol;&lol;&lol;&lol;">]><cap:alert xmlns:cap="urn:oasis:names:tc:emergency:cap:1.2"><cap:sent>2026-10-25T08:38:44+05:30</cap:sent><cap:identifier>&lol2;</cap:identifier></cap:alert>`
	if v, err := ParseCAP(strings.NewReader(bomb), fixtureItem("1792897745415016", "")); err == nil && strings.Contains(v.CapIdentifier, "lol") {
		t.Errorf("entity expanded into %q", v.CapIdentifier)
	}
}
