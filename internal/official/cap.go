package official

import (
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Severity is CAP's closed severity set. Anything else becomes SeverityUnknown.
type Severity string

const (
	SeverityExtreme  Severity = "Extreme"
	SeveritySevere   Severity = "Severe"
	SeverityModerate Severity = "Moderate"
	SeverityMinor    Severity = "Minor"
	SeverityUnknown  Severity = "Unknown"
)

// ParseSeverity maps a feed value onto the closed set, ignoring case and
// padding. The result is never a string the feed chose.
func ParseSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "extreme":
		return SeverityExtreme
	case "severe":
		return SeveritySevere
	case "moderate":
		return SeverityModerate
	case "minor":
		return SeverityMinor
	}
	return SeverityUnknown
}

// MsgType is the CAP message type. Only the three kept kinds have constants; a
// message of any other type (Ack, Error, anything unknown) carries its cleaned
// raw word and is not kept.
type MsgType string

const (
	MsgAlert  MsgType = "Alert"
	MsgUpdate MsgType = "Update"
	MsgCancel MsgType = "Cancel"
)

// The lifetime rule and the text limits. FallbackLifetime and MaxLifetime are
// the planner's two choices from research G2, pinned by TestExpiryRule.
const (
	FallbackLifetime = 6 * time.Hour

	MaxLifetime = 7 * 24 * time.Hour

	HeadlineMaxRunes    = 400
	DescriptionMaxRunes = 4000
	InstructionMaxRunes = 1000
	AreaMaxRunes        = 600
	IssuerMaxRunes      = 80
)

// Further bounds that keep one hostile message small.
const (
	maxDistrictCodes  = 800 // India has fewer than 800 districts
	maxDistrictValue  = 99999
	maxReferences     = 20
	maxReferenceRunes = 120
	maxLabelRunes     = 40
)

// Version is one normalised CAP message: one version of one alert. Every string
// is untrusted feed text that has been through CleanText, and nothing here is
// HTML.
type Version struct {
	GUID          string // the RSS guid, the series key
	CapIdentifier string // the message's own identifier, such as IN-1792919196941008_8
	Sender        string
	IssuerLabel   string // the RSS author's bracketed part, else the sender
	SentAt        time.Time

	Status     string
	Scope      string
	MsgType    MsgType
	References []string // full identifiers this message replaces or cancels

	Event     string
	Severity  Severity
	Urgency   string
	Certainty string

	EffectiveAt     *time.Time
	OnsetAt         *time.Time
	ExpiresAt       time.Time
	ExpiresInferred bool

	HeadlineEN          string
	HeadlineOriginal    string
	OriginalLang        string // the raw label (TL, MR), never parsed as BCP 47
	DescriptionEN       string
	DescriptionOriginal string
	Instruction         string
	AreaDesc            string
	DistrictCodes       []int // LGD district codes, sorted, unique

	SourcePubDate time.Time // the RSS item's pubDate when this was fetched
}

// Keep reports whether the message may be shown: a real, public Alert, Update
// or Cancel. Exercise, Test, System and Draft statuses, any other scope, and the
// Ack and Error types are dropped.
func (v Version) Keep() bool {
	if !strings.EqualFold(strings.TrimSpace(v.Status), "Actual") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(v.Scope), "Public") {
		return false
	}
	switch v.MsgType {
	case MsgAlert, MsgUpdate, MsgCancel:
		return true
	}
	return false
}

type capAlert struct {
	XMLName    xml.Name  `xml:"alert"`
	Identifier string    `xml:"identifier"`
	Sender     string    `xml:"sender"`
	Sent       string    `xml:"sent"`
	Status     string    `xml:"status"`
	MsgType    string    `xml:"msgType"`
	Scope      string    `xml:"scope"`
	References string    `xml:"references"`
	Infos      []capInfo `xml:"info"`
}

type capInfo struct {
	Language  string    `xml:"language"`
	Event     string    `xml:"event"`
	Urgency   string    `xml:"urgency"`
	Severity  string    `xml:"severity"`
	Certainty string    `xml:"certainty"`
	Effective string    `xml:"effective"`
	Onset     string    `xml:"onset"`
	Expires   string    `xml:"expires"`
	Headline  string    `xml:"headline"`
	Desc      string    `xml:"description"`
	Instr     string    `xml:"instruction"`
	Areas     []capArea `xml:"area"`
}

type capArea struct {
	Desc     string       `xml:"areaDesc"`
	Geocodes []capGeocode `xml:"geocode"`
}

type capGeocode struct {
	Name  string `xml:"valueName"`
	Value string `xml:"value"`
}

// ParseCAP reads one CAP 1.2 message into a Version. item supplies the RSS side:
// the guid, the issuer label and the pubDate. Tags are matched without their
// cap: namespace. encoding/xml resolves no external entities and leaves an
// undefined entity as an error, so a hostile DOCTYPE expands to nothing.
//
// The caller decides what to do with a message that is not Keep(); ParseCAP
// still returns it so a Cancel or an Exercise can be told apart from garbage.
func ParseCAP(r io.Reader, item Item) (Version, error) {
	var a capAlert
	if err := xml.NewDecoder(r).Decode(&a); err != nil {
		return Version{}, fmt.Errorf("%w: cap: %s", ErrParse, shortXMLError(err))
	}
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil {
		return Version{}, fmt.Errorf("%w: cap: sent time missing or invalid", ErrParse)
	}

	// The English block carries the event fields when there is one; otherwise
	// the first block does. The original is the first block that is not English.
	var en, orig *capInfo
	for i := range a.Infos {
		in := &a.Infos[i]
		if isEnglish(in.Language) {
			if en == nil {
				en = in
			}
		} else if orig == nil {
			orig = in
		}
	}
	var pick *capInfo
	switch {
	case en != nil:
		pick = en
	case len(a.Infos) > 0:
		pick = &a.Infos[0]
	default:
		pick = &capInfo{}
	}

	v := Version{
		GUID:          item.GUID,
		CapIdentifier: CleanText(a.Identifier, maxReferenceRunes),
		Sender:        CleanText(a.Sender, IssuerMaxRunes),
		IssuerLabel:   CleanText(item.IssuerLabel, IssuerMaxRunes),
		SentAt:        sent.UTC(),
		Status:        CleanText(a.Status, maxLabelRunes),
		Scope:         CleanText(a.Scope, maxLabelRunes),
		MsgType:       parseMsgType(a.MsgType),
		References:    parseReferences(a.References),
		Event:         CleanText(pick.Event, maxLabelRunes*2),
		Severity:      ParseSeverity(pick.Severity),
		Urgency:       CleanText(pick.Urgency, maxLabelRunes),
		Certainty:     CleanText(pick.Certainty, maxLabelRunes),
		EffectiveAt:   parseOptionalTime(pick.Effective),
		OnsetAt:       parseOptionalTime(pick.Onset),
		Instruction:   CleanText(pick.Instr, InstructionMaxRunes),
		AreaDesc:      joinAreas(pick.Areas),
		DistrictCodes: districtCodes(a.Infos),
		SourcePubDate: item.PubDate,
	}
	if v.IssuerLabel == "" {
		v.IssuerLabel = v.Sender
	}
	v.ExpiresAt, v.ExpiresInferred = resolveExpiry(v.SentAt, pick.Expires)
	if en != nil {
		v.HeadlineEN = CleanText(en.Headline, HeadlineMaxRunes)
		v.DescriptionEN = CleanText(en.Desc, DescriptionMaxRunes)
	}
	if orig != nil {
		v.HeadlineOriginal = CleanText(orig.Headline, HeadlineMaxRunes)
		v.DescriptionOriginal = CleanText(orig.Desc, DescriptionMaxRunes)
		v.OriginalLang = CleanText(orig.Language, maxLabelRunes)
	}
	return v, nil
}

// isEnglish is true for a language label that starts with "en", ignoring case
// and padding (en, en-IN, EN-in). Other labels, such as TL and MR, are opaque.
func isEnglish(lang string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "en")
}

func parseMsgType(s string) MsgType {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "alert":
		return MsgAlert
	case "update":
		return MsgUpdate
	case "cancel":
		return MsgCancel
	}
	return MsgType(CleanText(s, maxLabelRunes))
}

// parseReferences reads CAP's space separated sender,identifier,sent triples
// and returns the full identifiers, in order, once each. A lone token with no
// comma is taken as an identifier.
func parseReferences(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tok := range strings.Fields(s) {
		parts := strings.Split(tok, ",")
		var id string
		switch {
		case len(parts) >= 3:
			id = parts[len(parts)-2]
		case len(parts) == 1:
			id = parts[0]
		}
		id = CleanText(id, maxReferenceRunes)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
		if len(out) == maxReferences {
			break
		}
	}
	return out
}

func parseOptionalTime(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// resolveExpiry applies the lifetime rule. A valid expires is one that parses
// and lies between sent and sent plus MaxLifetime, both ends allowed. Anything
// else, including a missing value, ends at sent plus FallbackLifetime and is
// reported as inferred, so every alert disappears on its own.
func resolveExpiry(sent time.Time, raw string) (time.Time, bool) {
	exp, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil || exp.Before(sent) || exp.After(sent.Add(MaxLifetime)) {
		return sent.Add(FallbackLifetime).UTC(), true
	}
	return exp.UTC(), false
}

func joinAreas(areas []capArea) string {
	var descs []string
	for _, a := range areas {
		if d := CleanText(a.Desc, AreaMaxRunes); d != "" {
			descs = append(descs, d)
		}
	}
	return CleanText(strings.Join(descs, "; "), AreaMaxRunes)
}

// districtCodes collects the LGD District Code geocodes of every block, since
// the language blocks of one message name the same districts, as sorted unique
// integers. Values that are not small positive integers are skipped.
func districtCodes(infos []capInfo) []int {
	seen := map[int]bool{}
	for _, in := range infos {
		for _, area := range in.Areas {
			for _, g := range area.Geocodes {
				if !strings.EqualFold(strings.TrimSpace(g.Name), "LGD District Code") {
					continue
				}
				n, err := strconv.Atoi(strings.TrimSpace(g.Value))
				if err != nil || n < 1 || n > maxDistrictValue {
					continue
				}
				seen[n] = true
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	codes := make([]int, 0, len(seen))
	for n := range seen {
		codes = append(codes, n)
	}
	sort.Ints(codes)
	if len(codes) > maxDistrictCodes {
		codes = codes[:maxDistrictCodes]
	}
	return codes
}

// CleanText makes feed text safe to store and to put on a page as text. It
// drops invalid UTF-8, turns CRLF and lone CR into LF and a tab into a space,
// removes every other control character and the bidirectional override and
// isolate characters, trims, and cuts the result to maxRunes at a rune
// boundary. It keeps zero width joiners, which Indic scripts need. It does not
// touch markup: "<b>x</b>" stays literal text, and escaping is the job of
// html/template or textContent at render time.
func CleanText(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	s = strings.ToValidUTF8(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == 0x0a || r == 0x0d || r == 0x2028 || r == 0x2029:
			b.WriteByte('\n')
		case r == '\t':
			b.WriteByte(' ')
		case unicode.IsControl(r):
			// dropped
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0xfeff:
			// bidirectional overrides and the byte order mark: dropped
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > maxRunes {
		cut, n := 0, 0
		for i := range out {
			if n == maxRunes {
				cut = i
				break
			}
			n++
		}
		out = strings.TrimSpace(out[:cut])
	}
	return out
}
