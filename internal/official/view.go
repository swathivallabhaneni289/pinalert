package official

// The reader view: a pure function from the stored alerts, the feed status and
// the reader's and home areas to everything the Official alerts page and the
// Nearby banner show. The server authors every fact (the state, the labels, the
// count, the districts line, each row's level word and badge, the banner pick)
// so the browser recomputes none of it (D-06, D-08, research Pattern 2).
//
// BuildView does no I/O, reads no clock (Now comes in) and writes no log, so
// the Demo (plan 05-19) can feed it sample alerts and get the same answer shape.
// Feed text only travels in the fields below; the banner phrase comes from a
// closed table and never from feed text.

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"pinalert/internal/areas"
	"pinalert/internal/service"
)

// Group kinds, in the order the page draws them.
const (
	GroupReader    = "reader"
	GroupHome      = "home"
	GroupState     = "state"
	GroupUnplaced  = "unplaced"
	sourceSachet   = "sachet"
	sourceGdacs    = "gdacs"
	badgeOfficial  = "Official"
	badgeGdacs     = "GDACS"
	sourceLineNDMA = "NDMA SACHET"
	sourceLineGDAC = "GDACS. Automatic estimate. Follow your state disaster management authority."
	sourceLineDemo = "Made up for the Demo. Not from any issuer."
	unplacedLabel  = "Could not be placed on a map"
)

// areaTextMaxRunes bounds Row.AreaText; the full text stays in Details.Areas.
const areaTextMaxRunes = 160

// StoredAlert is one alert in force as stored: the columns of the in force read
// with the district and state codes of its area rows. DistrictNames holds the
// boundary data name of each DistrictCodes entry (same order), filled by the
// caller so this package's view needs no lookup. Source is "sachet" when empty.
type StoredAlert struct {
	ID     int64
	Source string

	Sender   string
	Issuer   string
	Event    string
	Severity string

	SentAt          time.Time
	ExpiresAt       time.Time
	ExpiresInferred bool

	HeadlineEN          string
	HeadlineOriginal    string
	OriginalLang        string
	DescriptionEN       string
	DescriptionOriginal string
	Instruction         string
	AreaDesc            string

	Tier          Tier
	DistrictCodes []int
	DistrictNames []string
	StateCodes    []int
}

// Area is where a person is, in our own boundary data names. DistrictCodes can
// hold several districts (a home cell tested at its centre and corners); the
// name and state are those of the first.
type Area struct {
	DistrictCodes []int
	DistrictName  string
	StateCode     int
	StateName     string
}

// ViewInput is everything BuildView reads. Reader is nil when the page sent no
// position (or it fell outside every district), Home when the account has no
// usable home area. Demo marks every row as a sample.
type ViewInput struct {
	Now    time.Time
	Status FeedStatus
	Reader *Area
	Home   *Area
	Alerts []StoredAlert
	Demo   bool
}

// View is the answer. Groups is never nil. AgeSeconds and LastSuccessAt are nil
// for a feed that never succeeded.
type View struct {
	State         FreshState
	AgeSeconds    *int64
	LastSuccessAt *time.Time
	AreaKnown     bool
	Groups        []Group
	Count         int
	DistrictsLine string
	Banner        *Banner
}

// AreaOfDistrict is the Area of one district in our own boundary data names,
// the same one the service builds for a reader's position. The Demo (plan
// 05-19) uses it to place its sample alerts on the pretend position's district.
func AreaOfDistrict(d areas.District) *Area { return districtArea(d) }

// Group is one labelled section of rows. Kind is reader, home, state or
// unplaced.
type Group struct {
	Kind  string
	Label string
	Rows  []Row
}

// Row is one alert, or one run of near identical alerts shown as the newest
// with GroupedCount above 1.
type Row struct {
	ID                   int64
	Source               string
	Level                string // extreme, severe, moderate, minor or unknown
	LevelWord            string
	Badge                string // empty for a Demo row
	Demo                 bool
	Headline             string
	OriginalLanguageOnly bool
	AreaText             string
	Issuer               string
	IssuedAt             time.Time
	ExpiresAt            time.Time
	ExpiresInferred      bool
	GroupedCount         int
	Details              Details
}

// Details is the disclosure panel of a row. Every part is empty when the feed
// did not carry it.
type Details struct {
	Description         string
	Instruction         string
	OriginalHeadline    string
	OriginalDescription string
	OriginalLangLabel   string
	OriginalLangCode    string
	Areas               string
	SourceLine          string
}

// Banner is the single line on Nearby: the most serious eligible alert for the
// reader's area and how many other eligible rows there are.
type Banner struct {
	Level       string
	LevelWord   string
	EventPhrase string
	Area        string
	More        int
	AlertID     int64
	// Demo is true when the view was built for the Demo (ViewInput.Demo): the
	// banner then draws the Demo tag and the alert is a made up sample.
	Demo bool
}

// entry is a row together with what the banner rule needs and the row does not
// carry.
type entry struct {
	row   Row
	event string
	rank  int
}

type section struct {
	kind    string
	label   string
	state   int
	entries []entry
}

// BuildView turns stored alerts into the honest answer for one reader. It
// sends no groups when the feed is unavailable or when neither a reader area
// nor a home area is known, and it sends a banner only when the feed is fresh.
func BuildView(in ViewInput) View {
	state, age := State(in.Status.LastSuccessAt, in.Now)
	v := View{
		State:     state,
		AreaKnown: in.Reader != nil || in.Home != nil,
		Groups:    []Group{},
	}
	if in.Status.LastSuccessAt != nil && !in.Status.LastSuccessAt.IsZero() {
		t := in.Status.LastSuccessAt.UTC()
		secs := int64(age / time.Second)
		v.LastSuccessAt, v.AgeSeconds = &t, &secs
	}
	if state == StateUnavailable || !v.AreaKnown {
		return v
	}

	homeShown := in.Home != nil && (in.Reader == nil || !subset(in.Home.DistrictCodes, in.Reader.DistrictCodes))

	var readerSec, homeSec, unplacedSec section
	var stateSecs []*section
	readerSec = section{kind: GroupReader}
	homeSec = section{kind: GroupHome}
	unplacedSec = section{kind: GroupUnplaced, label: unplacedLabel}
	if in.Reader != nil {
		readerSec.label = "Where you are: " + districtWords(in.Reader.DistrictName)
	}
	if homeShown {
		homeSec.label = "Your home area: " + districtWords(in.Home.DistrictName)
	}
	addState := func(a *Area) *section {
		for _, s := range stateSecs {
			if s.state == a.StateCode {
				return s
			}
		}
		s := &section{kind: GroupState, state: a.StateCode, label: "Across " + a.StateName}
		stateSecs = append(stateSecs, s)
		return s
	}

	// Each alert lands in exactly one section, the first that matches.
	buckets := map[*section][]StoredAlert{}
	var listed []StoredAlert
	for _, a := range in.Alerts {
		if !a.ExpiresAt.After(in.Now) {
			continue // an alert is gone by its own expiry in every state
		}
		var target *section
		switch a.Tier {
		case TierDistrict:
			switch {
			case in.Reader != nil && intersects(a.DistrictCodes, in.Reader.DistrictCodes):
				target = &readerSec
			case homeShown && intersects(a.DistrictCodes, in.Home.DistrictCodes):
				target = &homeSec
			}
		case TierState:
			switch {
			case in.Reader != nil && contains(a.StateCodes, in.Reader.StateCode):
				target = addState(in.Reader)
			case in.Home != nil && contains(a.StateCodes, in.Home.StateCode):
				target = addState(in.Home)
			}
		case TierUnplaced:
			target = &unplacedSec
		}
		if target == nil {
			continue
		}
		buckets[target] = append(buckets[target], a)
		listed = append(listed, a)
	}

	// The state sections keep the reader's state first, then the home's.
	ordered := []*section{&readerSec, &homeSec}
	sort.SliceStable(stateSecs, func(i, j int) bool {
		return in.Reader != nil && stateSecs[i].state == in.Reader.StateCode && stateSecs[j].state != in.Reader.StateCode
	})
	ordered = append(ordered, stateSecs...)
	ordered = append(ordered, &unplacedSec)

	for _, sec := range ordered {
		alerts := buckets[sec]
		if len(alerts) == 0 {
			continue
		}
		sec.entries = groupRows(alerts, in.Demo)
		rows := make([]Row, len(sec.entries))
		for i, e := range sec.entries {
			rows[i] = e.row
		}
		v.Groups = append(v.Groups, Group{Kind: sec.kind, Label: sec.label, Rows: rows})
		v.Count += len(rows)
	}

	v.DistrictsLine = districtsLine(districtNames(in, homeShown, listed))
	if state == StateFresh {
		v.Banner = pickBanner(in, &readerSec, stateSecs)
	}
	return v
}

// groupRows folds near identical alerts of one section into one entry and puts
// the entries in display order. The group key is sender, event, severity and the
// English headline (the original headline when there is no English one). The
// shown alert is the newest; the end time is the latest of the run.
func groupRows(alerts []StoredAlert, demo bool) []entry {
	type run struct {
		newest StoredAlert
		count  int
		latest StoredAlert // the alert with the latest end time
	}
	var order []string
	runs := map[string]*run{}
	for _, a := range alerts {
		headline := a.HeadlineEN
		if headline == "" {
			headline = a.HeadlineOriginal
		}
		key := strings.Join([]string{a.Sender, a.Event, a.Severity, headline}, "\x00")
		r, ok := runs[key]
		if !ok {
			runs[key] = &run{newest: a, latest: a, count: 1}
			order = append(order, key)
			continue
		}
		r.count++
		if newer(a, r.newest) {
			r.newest = a
		}
		if a.ExpiresAt.After(r.latest.ExpiresAt) {
			r.latest = a
		}
	}

	out := make([]entry, 0, len(order))
	for _, key := range order {
		r := runs[key]
		row := buildRow(r.newest, demo)
		row.GroupedCount = r.count
		row.ExpiresAt = r.latest.ExpiresAt
		row.ExpiresInferred = r.latest.ExpiresInferred
		out = append(out, entry{row: row, event: r.newest.Event, rank: severityRank(r.newest.Severity)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if !a.row.IssuedAt.Equal(b.row.IssuedAt) {
			return a.row.IssuedAt.After(b.row.IssuedAt)
		}
		return a.row.ID > b.row.ID
	})
	return out
}

func newer(a, b StoredAlert) bool {
	if !a.SentAt.Equal(b.SentAt) {
		return a.SentAt.After(b.SentAt)
	}
	return a.ID > b.ID
}

func severityRank(severity string) int {
	switch ParseSeverity(severity) {
	case SeverityExtreme:
		return 0
	case SeveritySevere:
		return 1
	case SeverityModerate:
		return 2
	case SeverityMinor:
		return 3
	}
	return 4
}

func buildRow(a StoredAlert, demo bool) Row {
	source := a.Source
	if source == "" {
		source = sourceSachet
	}
	badge, sourceLine := badgeOfficial, sourceLineNDMA
	if source == sourceGdacs {
		badge, sourceLine = badgeGdacs, sourceLineGDAC
	}
	if demo {
		// A Demo row has no authority badge and does not name an issuer's
		// channel as its source: nothing in it was ever sent by anyone
		// (T-05-99, ROBUST-06).
		badge, sourceLine = "", sourceLineDemo
	}
	issuer := a.Issuer
	if issuer == "" {
		issuer = a.Sender
	}

	headline := strings.TrimSpace(a.HeadlineEN)
	originalOnly := false
	switch {
	case headline != "":
	case strings.TrimSpace(a.HeadlineOriginal) != "":
		headline, originalOnly = strings.TrimSpace(a.HeadlineOriginal), true
	case strings.TrimSpace(a.Event) != "":
		headline = strings.TrimSpace(a.Event)
	default:
		headline = service.OfficialEventWord("")
	}

	d := Details{
		Description: a.DescriptionEN,
		Instruction: a.Instruction,
		Areas:       a.AreaDesc,
		SourceLine:  sourceLine,
	}
	if !originalOnly && a.HeadlineOriginal != "" && a.HeadlineOriginal != a.HeadlineEN {
		d.OriginalHeadline = a.HeadlineOriginal
	}
	if a.DescriptionOriginal != "" && a.DescriptionOriginal != a.DescriptionEN {
		d.OriginalDescription = a.DescriptionOriginal
	}
	if d.OriginalHeadline != "" || d.OriginalDescription != "" || originalOnly {
		d.OriginalLangLabel, d.OriginalLangCode = langLabel(a.OriginalLang)
	}

	level := ParseSeverity(a.Severity)
	return Row{
		ID:                   a.ID,
		Source:               source,
		Level:                strings.ToLower(string(level)),
		LevelWord:            service.OfficialLevelWord(a.Severity),
		Badge:                badge,
		Demo:                 demo,
		Headline:             headline,
		OriginalLanguageOnly: originalOnly,
		AreaText:             areaText(a.AreaDesc),
		Issuer:               issuer,
		IssuedAt:             a.SentAt,
		ExpiresAt:            a.ExpiresAt,
		ExpiresInferred:      a.ExpiresInferred,
		GroupedCount:         1,
		Details:              d,
	}
}

// areaText collapses the spacing of the cleaned area text and cuts it at
// areaTextMaxRunes with an ellipsis. The full text stays in Details.Areas.
func areaText(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= areaTextMaxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:areaTextMaxRunes])) + "…"
}

// pickBanner applies the D-16 rule. Eligible: a Severe or Extreme row placed on
// the reader's district, and an Extreme row across the reader's own state. The
// home area, unplaced alerts and a state only Severe alert never qualify. The
// most serious is picked, then the newest; More counts the other eligible rows
// (a grouped row counts once). Nil without a reader.
func pickBanner(in ViewInput, reader *section, stateSecs []*section) *Banner {
	if in.Reader == nil {
		return nil
	}
	type candidate struct {
		e    entry
		area string
	}
	var cands []candidate
	for _, e := range reader.entries {
		if e.rank <= 1 {
			cands = append(cands, candidate{e, districtWords(in.Reader.DistrictName)})
		}
	}
	for _, s := range stateSecs {
		if s.state != in.Reader.StateCode {
			continue
		}
		for _, e := range s.entries {
			if e.rank == 0 {
				cands = append(cands, candidate{e, in.Reader.StateName})
			}
		}
	}
	if len(cands) == 0 {
		return nil
	}
	best := 0
	for i := 1; i < len(cands); i++ {
		a, b := cands[i].e, cands[best].e
		switch {
		case a.rank != b.rank:
			if a.rank < b.rank {
				best = i
			}
		case !a.row.IssuedAt.Equal(b.row.IssuedAt):
			if a.row.IssuedAt.After(b.row.IssuedAt) {
				best = i
			}
		case a.row.ID > b.row.ID:
			best = i
		}
	}
	c := cands[best]
	return &Banner{
		Level:       c.e.row.Level,
		LevelWord:   c.e.row.LevelWord,
		EventPhrase: service.OfficialEventPhrase(c.e.event),
		Area:        c.area,
		More:        len(cands) - 1,
		AlertID:     c.e.row.ID,
		Demo:        in.Demo,
	}
}

// districtNames lists the districts the page covers: the reader's district,
// then the home district when its group is drawn, then the other districts the
// listed district alerts name, by code, without repeats.
func districtNames(in ViewInput, homeShown bool, listed []StoredAlert) []string {
	var names []string
	seen := map[string]bool{}
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	if in.Reader != nil {
		add(in.Reader.DistrictName)
	}
	if homeShown {
		add(in.Home.DistrictName)
	}
	byCode := map[int]string{}
	for _, a := range listed {
		if a.Tier != TierDistrict || len(a.DistrictNames) != len(a.DistrictCodes) {
			continue
		}
		for i, c := range a.DistrictCodes {
			byCode[c] = a.DistrictNames[i]
		}
	}
	codes := make([]int, 0, len(byCode))
	for c := range byCode {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	for _, c := range codes {
		add(byCode[c])
	}
	return names
}

// districtsLine reads "Krishna district" for one name, "A and B" for two,
// "A, B and C" for three and "A, B, C and 4 more" past three.
func districtsLine(names []string) string {
	switch n := len(names); {
	case n == 0:
		return ""
	case n == 1:
		return districtWords(names[0])
	case n == 2:
		return names[0] + " and " + names[1]
	case n == 3:
		return names[0] + ", " + names[1] + " and " + names[2]
	default:
		return strings.Join(names[:3], ", ") + " and " + strconv.Itoa(n-3) + " more"
	}
}

// districtWords is "<name> district", or the bare word when the name is empty.
func districtWords(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "district"
	}
	return name + " district"
}

// langLabel maps the feed's own language label (TL, MR, en-IN) to a language
// name and a valid BCP 47 code. The feed's labels are not BCP 47, so only the
// known ones get a code; an unknown label is kept as it is with no code.
func langLabel(raw string) (label, code string) {
	raw = strings.TrimSpace(raw)
	switch strings.ToLower(raw) {
	case "":
		return "", ""
	case "tl":
		return "Telugu", "te"
	case "mr":
		return "Marathi", "mr"
	case "ml":
		return "Malayalam", "ml"
	case "hi":
		return "Hindi", "hi"
	case "ta":
		return "Tamil", "ta"
	case "en", "en-in":
		return "English", "en"
	}
	return raw, ""
}

// titleName turns the boundary file's upper case state name into the words the
// page shows: each word capitalised, "and" and "of" kept lower case, "&" kept.
func titleName(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if i > 0 && (w == "and" || w == "of") {
			continue
		}
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func intersects(a, b []int) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

// subset reports whether every element of a is in b.
func subset(a, b []int) bool {
	for _, x := range a {
		if !contains(b, x) {
			return false
		}
	}
	return true
}
