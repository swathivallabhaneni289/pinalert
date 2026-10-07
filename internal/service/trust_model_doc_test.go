// trust_model_doc_test.go keeps docs/TRUST-MODEL.md true (D-03, D-08). The
// document makes claims about numbers, names, a refusal sentence and the
// tests that pin them. This test reads the document and compares each claim
// with the code as it stands, so a change to a constant, a renamed function
// or a test that no longer exists fails the build until the document is
// updated with it.
//
// The file is package service (the internal test package) so it can read
// the unexported voterGeohashPrecision. It recomputes no trust value: it only
// compares text in the document with compiled constants and with source
// files. Every helper and package variable starts with p20 so it cannot
// collide with a name from another plan. No database is needed.

package service

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// p20DocPath is the document under test, relative to this package.
const p20DocPath = "../../docs/TRUST-MODEL.md"

// p20Headings is the required heading order. Each must occur exactly once as
// a whole line. Other headings (the subsections of the still current and
// reporter sections) may sit between them.
var p20Headings = []string{
	"# Pinalert trust model",
	"## What counts as a place",
	"### Derivation table",
	"### Width by city at precision 7",
	"### Witness simulation",
	"### Why 7",
	"## Who can vote: the 1 km rule",
	"## Votes cast before the 1 km rule",
	"## The two counters",
	"## Still current signal",
	"## Reporter reliability",
	"## One trust object on every surface",
	"## Browser hints, server decisions",
	"## Where the roadmap words are met",
	"## Hand offs to later phases",
	"## Keeping this document true",
	"## Honest limits",
	"## Out of scope for this model",
}

// p20ConstFiles names the file that declares each constant the document
// tables, with the repository path the table must also show.
var p20ConstFiles = map[string]string{
	"MaxVoteDistanceKm":             "internal/service/geo.go",
	"voterGeohashPrecision":         "internal/service/trust.go",
	"IndependentAgreementThreshold": "internal/service/visibility.go",
	"CurrencyWindowDivisor":         "internal/service/currency.go",
	"CurrencyPointsPerWindow":       "internal/service/currency.go",
	"UpToDateMinScore":              "internal/service/currency.go",
	"GettingOldMinScore":            "internal/service/currency.go",
	"ReliabilityMinJudged":          "internal/service/reliability.go",
	"ReliabilityHalfLife":           "internal/service/reliability.go",
	"ReliabilityWindow":             "internal/service/reliability.go",
	"ReliabilityRowCap":             "internal/service/reliability.go",
	"UnconfirmedCredit":             "internal/service/reliability.go",
	"ReliableMinScore":              "internal/service/reliability.go",
	"MixedMinScore":                 "internal/service/reliability.go",
	"MaxReporterAccountsPerRead":    "internal/service/report.go",
}

// p20Identifiers lists names the document must show in backticks and that
// must be declared in the named file.
var p20Identifiers = []struct{ name, file string }{
	{"ErrVoterTooFar", "internal/service/geo.go"},
	{"TooFarMessage", "internal/service/geo.go"},
	{"haversineKm", "internal/service/geo.go"},
	{"withinVoteRadius", "internal/service/geo.go"},
	{"filterVoteRows", "internal/service/trust.go"},
	{"BuildVoteTally", "internal/service/trust.go"},
	{"BuildTrust", "internal/service/trustview.go"},
	{"ClassifyOutcome", "internal/service/reliability.go"},
	{"ScoreReliability", "internal/service/reliability.go"},
	{"ReporterSignals", "internal/service/reporter.go"},
	{"trustToResponse", "internal/api/handlers/trust.go"},
}

// p20MustCite is the set of tests the document has to name.
var p20MustCite = []string{
	"TestHaversine",
	"TestWithinVoteRadiusBoundary",
	"TestCastVoteRejectsTooFar",
	"TestCastVoteCheckOrder",
	"TestCastVoteReporterExemption",
	"TestCastVoteTooFar",
	"TestGeohashPrecisionWitnesses",
	"TestBuildTrustCountsMatchTally",
	"TestTrustCountLabel",
	"TestTrustCountsDistinctPlacesNotVotes",
	"TestTrustCountUnderConcurrentVotes",
	"TestFeedTrustMatchesCastVoteTrust",
	"TestTrustResponseOmitsIdentifiers",
	"TestTrustAttachedOnEveryReadPath",
	"TestCurrencyScoreBoundaries",
	"TestReliabilityTable",
	"TestReliabilityInvariants",
	"TestReporterSignalsGroupByAccount",
	"TestNearbyBatchesVoteReadsOnceWithReporters",
	"TestNearbyReporterAccountCap",
	"TestNearbyRatingNeverChangesVisibilityOrOrder",
	"TestFeedSignalsMatchCastVoteSignals",
	"TestReporterSignalFromSeededHistory",
	"TestProfileRowsCarryTrustJSON",
	"TestVoteTransportRefreshesAndRetriesOnce",
	"TestVoteHint",
	"TestPageShellCarriesVoteRadius",
	"TestTrustModuleHasNoThresholdsOrSinks",
	"TestTrustModelDocMatchesCode",
}

// p20NoSpoofClaims are phrases the document must never contain. A spoofed
// location defeats the model, and the document says so (T-03-03).
var p20NoSpoofClaims = []string{
	"spoof-proof",
	"spoof proof",
	"spoofproof",
	"immune to spoofing",
	"prevents spoofing",
	"stops spoofing",
	"resistant to spoofing",
	"spoof-resistant",
	"spoof resistant",
	"resists spoofing",
}

var (
	p20ConstRow  = regexp.MustCompile("^\\|\\s*`([A-Za-z0-9_]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*([^|]+?)\\s*\\|\\s*$")
	p20TestToken = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)
	p20TestFunc  = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)
	p20TooFarMap = regexp.MustCompile(`writeFieldError\(\s*w,\s*http\.StatusForbidden,\s*"location",\s*service\.TooFarMessage\(\)\s*\)`)
)

// p20ReadFile reads a file relative to the repository root.
func p20ReadFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// p20ReadDoc reads docs/TRUST-MODEL.md.
func p20ReadDoc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(p20DocPath)
	if err != nil {
		t.Fatalf("read %s: %v", p20DocPath, err)
	}
	return string(b)
}

// p20Section returns the body of one second level section: the lines after
// the heading up to, not including, the next line that starts a second level
// heading. It returns false when the heading is absent.
func p20Section(doc, heading string) (string, bool) {
	lines := strings.Split(doc, "\n")
	start := -1
	for i, l := range lines {
		if l == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

// p20Days renders a duration that is a whole number of days as "{n} days".
func p20Days(t *testing.T, d time.Duration) string {
	t.Helper()
	day := 24 * time.Hour
	if d <= 0 || d%day != 0 {
		t.Fatalf("duration %v is not a positive whole number of days", d)
	}
	return strconv.FormatInt(int64(d/day), 10) + " days"
}

// p20Constants returns, for each tabled constant, the text its Value cell
// must show, written from the compiled value.
func p20Constants(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"MaxVoteDistanceKm":             strconv.FormatFloat(MaxVoteDistanceKm, 'f', -1, 64) + " km",
		"voterGeohashPrecision":         fmt.Sprint(voterGeohashPrecision),
		"IndependentAgreementThreshold": fmt.Sprint(IndependentAgreementThreshold),
		"CurrencyWindowDivisor":         fmt.Sprint(CurrencyWindowDivisor),
		"CurrencyPointsPerWindow":       fmt.Sprint(CurrencyPointsPerWindow),
		"UpToDateMinScore":              fmt.Sprint(UpToDateMinScore),
		"GettingOldMinScore":            fmt.Sprint(GettingOldMinScore),
		"ReliabilityMinJudged":          fmt.Sprint(ReliabilityMinJudged),
		"ReliabilityHalfLife":           p20Days(t, ReliabilityHalfLife),
		"ReliabilityWindow":             p20Days(t, ReliabilityWindow),
		"ReliabilityRowCap":             fmt.Sprint(ReliabilityRowCap),
		"UnconfirmedCredit":             fmt.Sprint(UnconfirmedCredit),
		"ReliableMinScore":              fmt.Sprint(ReliableMinScore),
		"MixedMinScore":                 fmt.Sprint(MixedMinScore),
		"MaxReporterAccountsPerRead":    fmt.Sprint(MaxReporterAccountsPerRead),
	}
}

// p20DeclRegexp matches a line that declares name: optional indentation, an
// optional const or var, the name as a whole word, then anything but a
// newline up to an equals sign. It covers single declarations and aligned
// rows inside a const block.
func p20DeclRegexp(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[ \t]*(?:(?:const|var)[ \t]+)?` + regexp.QuoteMeta(name) + `\b[^\n=]*=`)
}

// p20TestFuncNames collects the name of every Test function in every
// _test.go file under root. Directories whose name starts with a dot are
// skipped (the walk root itself excepted) so that agent worktrees kept under
// .claude cannot hide a missing test, and node_modules is skipped.
func p20TestFuncNames(t *testing.T, root string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range p20TestFunc.FindAllStringSubmatch(string(b), -1) {
			names[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return names
}

func TestTrustModelDocMatchesCode(t *testing.T) {
	doc := p20ReadDoc(t)

	t.Run("headings in order", func(t *testing.T) {
		lines := strings.Split(doc, "\n")
		prev := -1
		for _, h := range p20Headings {
			found := []int{}
			for i, l := range lines {
				if l == h {
					found = append(found, i)
				}
			}
			if len(found) != 1 {
				t.Errorf("heading %q occurs %d times, want exactly 1", h, len(found))
				continue
			}
			if found[0] <= prev {
				t.Errorf("heading %q is out of order (line %d, previous heading at line %d)", h, found[0]+1, prev+1)
			}
			prev = found[0]
		}
	})

	t.Run("constants table", func(t *testing.T) {
		body, ok := p20Section(doc, "## Keeping this document true")
		if !ok {
			t.Fatalf("section %q is missing", "## Keeping this document true")
		}
		expected := p20Constants(t)
		type row struct{ file, value string }
		rows := map[string][]row{}
		for _, l := range strings.Split(body, "\n") {
			if m := p20ConstRow.FindStringSubmatch(l); m != nil {
				rows[m[1]] = append(rows[m[1]], row{file: m[2], value: m[3]})
			}
		}
		for name := range rows {
			if _, known := expected[name]; !known {
				t.Errorf("constants table has a row for %s, which this test does not know", name)
			}
		}
		for name, want := range expected {
			got := rows[name]
			if len(got) != 1 {
				t.Errorf("constants table has %d rows for %s, want exactly 1", len(got), name)
				continue
			}
			if got[0].value != want {
				t.Errorf("constants table row %s shows %q, the code says %q", name, got[0].value, want)
			}
			file := p20ConstFiles[name]
			if got[0].file != file {
				t.Errorf("constants table row %s names file %q, want %q", name, got[0].file, file)
			}
			if !p20DeclRegexp(name).MatchString(p20ReadFile(t, file)) {
				t.Errorf("%s does not declare %s", file, name)
			}
		}
	})

	t.Run("named identifiers", func(t *testing.T) {
		for _, id := range p20Identifiers {
			if !strings.Contains(doc, "`"+id.name+"`") {
				t.Errorf("document does not name `%s` in backticks", id.name)
			}
			src := p20ReadFile(t, id.file)
			var declared bool
			if id.name == "ErrVoterTooFar" {
				declared = p20DeclRegexp(id.name).MatchString(src)
			} else {
				declared = regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(id.name) + `\(`).MatchString(src)
			}
			if !declared {
				t.Errorf("%s does not declare %s", id.file, id.name)
			}
		}
	})

	t.Run("too far refusal", func(t *testing.T) {
		if !strings.Contains(doc, "TooFarMessage()") {
			t.Errorf("document does not contain TooFarMessage() verbatim")
		}
		if !strings.Contains(doc, TooFarMessage()) {
			t.Errorf("document does not contain the refusal sentence %q", TooFarMessage())
		}
		votes := p20ReadFile(t, "internal/api/handlers/votes.go")
		if !strings.Contains(votes, "service.ErrVoterTooFar") {
			t.Errorf("votes.go does not test for service.ErrVoterTooFar")
		}
		if !p20TooFarMap.MatchString(votes) {
			t.Errorf("votes.go does not map the refusal to writeFieldError(w, http.StatusForbidden, \"location\", service.TooFarMessage())")
		}
	})

	t.Run("named tests exist", func(t *testing.T) {
		have := p20TestFuncNames(t, filepath.Join("..", ".."))
		seen := map[string]bool{}
		missing := []string{}
		for _, tok := range p20TestToken.FindAllString(doc, -1) {
			if seen[tok] {
				continue
			}
			seen[tok] = true
			if !have[tok] {
				missing = append(missing, tok)
			}
		}
		if len(missing) > 0 {
			t.Errorf("document names tests that do not exist as a func in any _test.go file: %s", strings.Join(missing, ", "))
		}
		notCited := []string{}
		for _, n := range p20MustCite {
			if !seen[n] {
				notCited = append(notCited, n)
			}
		}
		if len(notCited) > 0 {
			t.Errorf("document does not name these tests: %s", strings.Join(notCited, ", "))
		}
	})

	t.Run("plain text", func(t *testing.T) {
		for _, r := range []rune{rune(0x2013), rune(0x2014)} {
			if strings.ContainsRune(doc, r) {
				t.Errorf("document contains the long dash U+%04X", r)
			}
		}
		lower := strings.ToLower(doc)
		for _, phrase := range p20NoSpoofClaims {
			if strings.Contains(lower, phrase) {
				t.Errorf("document contains the spoof resistance claim %q", phrase)
			}
		}
	})
}
