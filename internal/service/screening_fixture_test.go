// screening_fixture_test.go is the D-07 fixture test (V-02): it judges every
// entry of testdata/screening/genuine.json and abusive.json offline, in the
// order of the inline check (the gibberish rule first, then Decide on the
// recorded provider scores).
//
// Honesty about what this proves (D-07). The texts were written for this test
// by the project, not taken from real traffic. If zero of N genuine entries
// are rejected, the true rate is probably under about 3 in N (the rule of
// three), not the one in a thousand design target. Real calibration needs real
// traffic and the moderator screen's decisions.
//
// Scores are recorded once by the owner with
// MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord against the
// pinned snapshot, and are never typed by hand. A text edited after recording
// must have its model and scores cleared so the recorder records it again.
//
// Known limits. One word texts are refused by the gibberish rule by design
// (RESEARCH A9), so every genuine text has at least two words. Romanised
// Telugu and Tamil fit none of the five language labels and are not in the
// fixture.
//
// Until scores are recorded TestScreeningFixture skips, and the offline half
// (structure, the gibberish rule, and the proof that the gate can fail) runs in
// every quick run. No test message prints fixture text, only ids, languages,
// verdicts, reasons and scores.

package service_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"pinalert/internal/service"
)

type screeningFixtureEntry struct {
	ID       string             `json:"id"`
	Text     string             `json:"text"`
	Language string             `json:"language"`
	Serious  bool               `json:"serious"`
	Expect   string             `json:"expect"`
	Model    string             `json:"model"`
	Scores   map[string]float64 `json:"scores"`
}

const (
	screeningFixtureSnapshot   = "omni-moderation-2024-09-26"
	screeningFixtureMinRunes   = 10
	screeningFixtureMaxRunes   = 1000
	screeningFixtureTerseRunes = 25
	screeningFixtureLongRunes  = 300
	screeningFixtureStretchMin = 4
)

var screeningFixtureLanguages = []string{"en", "hi", "te", "ta", "hinglish"}

// screeningFixtureRejectKeyNames are the six provider keys behind the three
// reject reasons; they are used only for logged margins.
var screeningFixtureRejectKeyNames = []string{
	"hate", "hate/threatening",
	"harassment", "harassment/threatening",
	"sexual", "sexual/minors",
}

var (
	screeningFixtureGenuineID     = regexp.MustCompile(`^g-(en|hi|te|ta|hinglish)-[0-9]{3}$`)
	screeningFixtureAbusiveID     = regexp.MustCompile(`^a-(hateful|harassing|sexual|gibberish)-(en|hi|te|ta|hinglish)-[0-9]{2}$`)
	screeningFixturePhone         = regexp.MustCompile(`\b[6-9][0-9]{4}[ -]?[0-9]{5}\b`)
	screeningFixtureAddress       = regexp.MustCompile(`(?i)\b(?:road|street|nagar|colony|cross|lane|marg|chowk|gali|sector|ward|junction|circle|bus stand|flat|plot|door no|nh ?[0-9]+)\b|\b[1-9][0-9]{5}\b`)
	screeningFixtureUrgent        = regexp.MustCompile(`(?i)!!|\b(?:urgent|urgently|sos|emergency|asap|immediately|jaldi|turant|please help|help us)\b`)
	screeningFixtureLink          = regexp.MustCompile(`https?://`)
	screeningFixtureHinglishWords = regexp.MustCompile(`(?i)\b(?:hai|hain|nahi|nahin|raha|rahe|rahi|mein|paani|pani|bhai|jaldi|kya|koi|ghar|abhi|chahiye|gaya|gayi|gaye|bahut|kuch|yahan|bijli|sadak|aur)\b`)
	screeningFixtureHurtWords     = map[string]*regexp.Regexp{
		"en":       regexp.MustCompile(`(?i)\b(?:dead|died|dying|death|body|bodies|blood|bleeding|injured|injury|drowned|drowning|kill|killed|crushed|electrocuted|suicide|maut|lash|zakhmi|khoon)\b`),
		"hinglish": regexp.MustCompile(`(?i)\b(?:dead|died|dying|death|body|bodies|blood|bleeding|injured|injury|drowned|drowning|kill|killed|crushed|electrocuted|suicide|maut|lash|zakhmi|khoon)\b`),
		"hi":       regexp.MustCompile(`मौत|घायल|लाश|खून|डूब`),
		"te":       regexp.MustCompile(`చనిపో|మృత|గాయపడ|రక్త|మునిగ`),
		"ta":       regexp.MustCompile(`இறந்|உயிரிழ|காயம|ரத்த|மூழ்க`),
	}
)

func screeningFixtureLoad(t *testing.T, name string) ([]screeningFixtureEntry, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "screening", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var entries []screeningFixtureEntry
	if err := dec.Decode(&entries); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("fixture %s has data after the array: %v", name, err)
	}
	return entries, raw
}

func screeningFixtureCanonical(entries []screeningFixtureEntry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// screeningFixtureDecision mirrors the inline check: the gibberish rule first,
// then Decide on the recorded scores. It is the only place the test calls Decide.
func screeningFixtureDecision(e screeningFixtureEntry) service.ScreenDecision {
	if service.IsGibberish(e.Text) {
		return service.ScreenDecision{Verdict: service.VerdictReject, Reason: service.RejectGibberish}
	}
	return service.Decide(e.Scores, e.Serious)
}

// screeningFixtureProblems lists, in input order, every entry whose decision
// breaks the fixture's promise: a genuine entry rejected (a serious one is
// "withheld"), or an abusive entry not rejected. Messages carry ids, languages,
// verdicts, reasons and scores only, never the text.
func screeningFixtureProblems(entries []screeningFixtureEntry) []string {
	var problems []string
	for _, e := range entries {
		d := screeningFixtureDecision(e)
		score := screeningFixtureMaxRejectScore(e.Scores)
		switch e.Expect {
		case "publish":
			if d.Verdict != service.VerdictReject {
				continue
			}
			if e.Serious {
				problems = append(problems, fmt.Sprintf("%s (%s) is serious and would be withheld: rejected as %s, highest reject score %.4f", e.ID, e.Language, d.Reason, score))
			} else {
				problems = append(problems, fmt.Sprintf("%s (%s) is genuine and was rejected as %s, highest reject score %.4f", e.ID, e.Language, d.Reason, score))
			}
		case "reject":
			if d.Verdict != service.VerdictReject {
				problems = append(problems, fmt.Sprintf("%s (%s) is abusive and was not rejected: verdict %s, highest reject score %.4f", e.ID, e.Language, d.Verdict, score))
			}
		}
	}
	return problems
}

func screeningFixtureModelProblems(entries []screeningFixtureEntry) []string {
	var problems []string
	for _, e := range entries {
		if e.Model != screeningFixtureSnapshot {
			problems = append(problems, fmt.Sprintf("%s recorded on model %q, want %q", e.ID, e.Model, screeningFixtureSnapshot))
		}
	}
	return problems
}

// screeningFixtureRecordState returns how many entries carry scores and the
// ids of those that do not, in input order.
func screeningFixtureRecordState(entries []screeningFixtureEntry) (int, []string) {
	recorded := 0
	var missing []string
	for _, e := range entries {
		if len(e.Scores) > 0 {
			recorded++
		} else {
			missing = append(missing, e.ID)
		}
	}
	return recorded, missing
}

func screeningFixtureScriptProblem(language, text string) string {
	var latin, devanagari, telugu, tamil, other int
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		switch {
		case unicode.Is(unicode.Latin, r):
			latin++
		case unicode.Is(unicode.Devanagari, r):
			devanagari++
		case unicode.Is(unicode.Telugu, r):
			telugu++
		case unicode.Is(unicode.Tamil, r):
			tamil++
		default:
			other++
		}
	}
	native := map[string]int{"hi": devanagari, "te": telugu, "ta": tamil}
	switch language {
	case "en", "hinglish":
		if devanagari+telugu+tamil+other > 0 {
			return "has letters outside the Latin script"
		}
		return ""
	case "hi", "te", "ta":
		own := native[language]
		foreign := devanagari + telugu + tamil + other - own
		switch {
		case own == 0:
			return "has no letter of its own script"
		case own <= latin:
			return "has no more letters of its own script than Latin letters"
		case foreign > 0:
			return "has letters of another script"
		}
		return ""
	}
	return "has an unknown language"
}

func screeningFixtureAllCapitals(text string) bool {
	letters := 0
	for _, r := range text {
		if !unicode.IsLetter(r) || !unicode.Is(unicode.Latin, r) {
			continue
		}
		if unicode.IsLower(r) {
			return false
		}
		letters++
	}
	return letters >= 10
}

func screeningFixtureStretched(text string) bool {
	var prev rune
	run := 0
	check := func() bool {
		return run >= screeningFixtureStretchMin && run < service.GibberishRunLength
	}
	for _, r := range text {
		if unicode.IsLetter(r) && run > 0 && r == prev {
			run++
		} else {
			if check() {
				return true
			}
			run = 0
			if unicode.IsLetter(r) {
				run = 1
			}
		}
		prev = r
	}
	return check()
}

func screeningFixtureMaxRejectScore(scores map[string]float64) float64 {
	var best float64
	for _, k := range screeningFixtureRejectKeyNames {
		if scores[k] > best {
			best = scores[k]
		}
	}
	return best
}

func screeningFixtureSynthetic(id, text string, serious bool, expect string, scores map[string]float64) screeningFixtureEntry {
	return screeningFixtureEntry{
		ID: id, Text: text, Language: "en", Serious: serious, Expect: expect,
		Model: screeningFixtureSnapshot, Scores: scores,
	}
}

func screeningFixtureAll(t *testing.T) ([]screeningFixtureEntry, []screeningFixtureEntry) {
	t.Helper()
	genuine, _ := screeningFixtureLoad(t, "genuine.json")
	abusive, _ := screeningFixtureLoad(t, "abusive.json")
	return genuine, abusive
}

func TestScreeningFixtureShape(t *testing.T) {
	t.Run("genuine file has the wording a careless filter would refuse", func(t *testing.T) {
		genuine, _ := screeningFixtureLoad(t, "genuine.json")
		if len(genuine) < 150 || len(genuine) > 300 {
			t.Fatalf("genuine has %d entries, want 150 to 300", len(genuine))
		}
		type tally struct{ n, serious, notSerious, phone, hurt, terse int }
		perLanguage := map[string]*tally{}
		for _, l := range screeningFixtureLanguages {
			perLanguage[l] = &tally{}
		}
		var phone, hurt, terse, long, caps, address, urgent, link, stretched int
		for _, e := range genuine {
			m := screeningFixtureGenuineID.FindStringSubmatch(e.ID)
			if m == nil {
				t.Errorf("%s: id does not match the genuine pattern", e.ID)
				continue
			}
			if m[1] != e.Language {
				t.Errorf("%s: language %q does not match the id", e.ID, e.Language)
			}
			if e.Expect != "publish" {
				t.Errorf("%s: expect is %q, want publish", e.ID, e.Expect)
			}
			if e.Language == "hinglish" && !screeningFixtureHinglishWords.MatchString(e.Text) {
				t.Errorf("%s: a hinglish entry holds no listed Hindi word", e.ID)
			}
			tl := perLanguage[m[1]]
			runes := utf8.RuneCountInString(e.Text)
			tl.n++
			if e.Serious {
				tl.serious++
			} else {
				tl.notSerious++
			}
			if screeningFixturePhone.MatchString(e.Text) {
				tl.phone++
				phone++
			}
			if re := screeningFixtureHurtWords[m[1]]; re != nil && re.MatchString(e.Text) {
				tl.hurt++
				hurt++
			}
			if runes <= screeningFixtureTerseRunes {
				tl.terse++
				terse++
			}
			if runes >= screeningFixtureLongRunes {
				long++
			}
			if screeningFixtureAllCapitals(e.Text) {
				caps++
			}
			if screeningFixtureAddress.MatchString(e.Text) {
				address++
			}
			if screeningFixtureUrgent.MatchString(e.Text) {
				urgent++
			}
			if screeningFixtureLink.MatchString(e.Text) {
				link++
			}
			if screeningFixtureStretched(e.Text) {
				stretched++
			}
		}
		for _, l := range screeningFixtureLanguages {
			tl := perLanguage[l]
			if tl.n < 30 || tl.serious < 8 || tl.notSerious < 8 || tl.phone < 2 || tl.hurt < 2 || tl.terse < 1 {
				t.Errorf("language %s is thin: %d entries, %d serious, %d not serious, %d phone, %d hurt wording, %d terse", l, tl.n, tl.serious, tl.notSerious, tl.phone, tl.hurt, tl.terse)
			}
		}
		floors := []struct {
			name      string
			got, want int
		}{
			{"mobile number", phone, 12}, {"death or injury wording", hurt, 10},
			{"terse", terse, 8}, {"long", long, 5}, {"all capitals", caps, 10},
			{"address", address, 20}, {"urgent wording", urgent, 20},
			{"link", link, 2}, {"stretched letter", stretched, 3},
		}
		for _, f := range floors {
			if f.got < f.want {
				t.Errorf("only %d %s entries, want at least %d", f.got, f.name, f.want)
			}
		}
		t.Logf("genuine coverage: %d phone, %d hurt, %d terse, %d long, %d capitals, %d address, %d urgent, %d link, %d stretched", phone, hurt, terse, long, caps, address, urgent, link, stretched)
	})

	t.Run("abusive file covers every reason and language", func(t *testing.T) {
		abusive, _ := screeningFixtureLoad(t, "abusive.json")
		if len(abusive) < 30 {
			t.Fatalf("abusive has %d entries, want at least 30", len(abusive))
		}
		type tally struct{ n, serious, notSerious int }
		type languageTally struct{ n, scoreBased int }
		perReason := map[string]*tally{}
		perLanguage := map[string]*languageTally{}
		for _, e := range abusive {
			m := screeningFixtureAbusiveID.FindStringSubmatch(e.ID)
			if m == nil {
				t.Errorf("%s: id does not match the abusive pattern", e.ID)
				continue
			}
			if !service.RejectReason(m[1]).Valid() {
				t.Errorf("%s: reason %q is not a reject reason", e.ID, m[1])
			}
			if m[2] != e.Language {
				t.Errorf("%s: language %q does not match the id", e.ID, e.Language)
			}
			if e.Expect != "reject" {
				t.Errorf("%s: expect is %q, want reject", e.ID, e.Expect)
			}
			if e.Language == "hinglish" && !screeningFixtureHinglishWords.MatchString(e.Text) {
				t.Errorf("%s: a hinglish entry holds no listed Hindi word", e.ID)
			}
			if perReason[m[1]] == nil {
				perReason[m[1]] = &tally{}
			}
			r := perReason[m[1]]
			r.n++
			if e.Serious {
				r.serious++
			} else {
				r.notSerious++
			}
			if perLanguage[m[2]] == nil {
				perLanguage[m[2]] = &languageTally{}
			}
			l := perLanguage[m[2]]
			l.n++
			if m[1] != string(service.RejectGibberish) {
				l.scoreBased++
			}
		}
		for _, reason := range service.RejectReasons {
			r := perReason[string(reason)]
			if r == nil || r.n < 6 || r.serious < 2 || r.notSerious < 2 {
				t.Errorf("reason %s is thin: %+v", reason, r)
			}
		}
		for _, l := range screeningFixtureLanguages {
			tl := perLanguage[l]
			if tl == nil || tl.n < 4 || tl.scoreBased < 2 {
				t.Errorf("language %s is thin: %+v", l, tl)
			}
		}
	})

	t.Run("both files are well formed and in the recorder layout", func(t *testing.T) {
		genuine, genuineRaw := screeningFixtureLoad(t, "genuine.json")
		abusive, abusiveRaw := screeningFixtureLoad(t, "abusive.json")
		ids := map[string]bool{}
		texts := map[string]bool{}
		for _, file := range []struct {
			name    string
			entries []screeningFixtureEntry
			raw     []byte
		}{{"genuine.json", genuine, genuineRaw}, {"abusive.json", abusive, abusiveRaw}} {
			for _, e := range file.entries {
				if ids[e.ID] {
					t.Errorf("%s: duplicate id", e.ID)
				}
				ids[e.ID] = true
				lower := strings.ToLower(e.Text)
				if texts[lower] {
					t.Errorf("%s: text repeats another entry", e.ID)
				}
				texts[lower] = true
				known := false
				for _, l := range screeningFixtureLanguages {
					known = known || l == e.Language
				}
				if !known {
					t.Errorf("%s: unknown language %q", e.ID, e.Language)
				}
				if e.Text != strings.TrimSpace(e.Text) {
					t.Errorf("%s: text is not trimmed", e.ID)
				}
				if !utf8.ValidString(e.Text) {
					t.Errorf("%s: text is not valid UTF-8", e.ID)
				}
				runes := utf8.RuneCountInString(e.Text)
				if runes < screeningFixtureMinRunes || runes > screeningFixtureMaxRunes {
					t.Errorf("%s: %d runes, want %d to %d", e.ID, runes, screeningFixtureMinRunes, screeningFixtureMaxRunes)
				}
				for _, r := range e.Text {
					if (r < 0x20 && r != '\n') || r == 0x2013 || r == 0x2014 {
						t.Errorf("%s: text holds a control character or a dash (U+%04X)", e.ID, r)
						break
					}
				}
				problem := screeningFixtureScriptProblem(e.Language, e.Text)
				if problem != "" {
					t.Errorf("%s: %s", e.ID, problem)
				}
				if e.Scores == nil {
					t.Errorf("%s: scores is null, want an object", e.ID)
				}
				if (e.Model == "") != (len(e.Scores) == 0) {
					t.Errorf("%s: model and scores must both be empty or both be set", e.ID)
				}
				for k, v := range e.Scores {
					if math.IsNaN(v) || v < 0 || v > 1 {
						t.Errorf("%s: score for %q is outside 0 to 1", e.ID, k)
					}
				}
			}
			want, err := screeningFixtureCanonical(file.entries)
			if err != nil {
				t.Fatalf("%s: re-encode: %v", file.name, err)
			}
			if !bytes.Equal(want, file.raw) {
				t.Errorf("%s is not in the recorder's exact layout", file.name)
			}
		}
	})
}

func TestScreeningFixtureGibberishRule(t *testing.T) {
	genuine, abusive := screeningFixtureAll(t)
	var wrongGenuine, missedGibberish, wrongAbusive []string
	for _, e := range genuine {
		if service.IsGibberish(e.Text) {
			wrongGenuine = append(wrongGenuine, e.ID)
		}
	}
	for _, e := range abusive {
		isGibberishID := strings.HasPrefix(e.ID, "a-gibberish-")
		got := service.IsGibberish(e.Text)
		switch {
		case isGibberishID && !got:
			missedGibberish = append(missedGibberish, e.ID)
		case !isGibberishID && got:
			wrongAbusive = append(wrongAbusive, e.ID)
		}
	}
	if len(wrongGenuine) > 0 {
		t.Errorf("genuine entries that are gibberish: %s", strings.Join(wrongGenuine, ", "))
	}
	if len(missedGibberish) > 0 {
		t.Errorf("gibberish examples the rule does not catch: %s", strings.Join(missedGibberish, ", "))
	}
	if len(wrongAbusive) > 0 {
		t.Errorf("score based abusive entries that the gibberish rule already refuses: %s", strings.Join(wrongAbusive, ", "))
	}
}

func TestScreeningFixtureGateCatchesFailures(t *testing.T) {
	reject := service.RejectScoreThreshold
	borderline := service.BorderlineScoreThreshold
	justBelow := math.Nextafter(service.RejectScoreThreshold, 0)

	clean := []screeningFixtureEntry{
		screeningFixtureSynthetic("syn-clean-1", "The road near the bridge is flooded", false, "publish", map[string]float64{"violence": 1, "self-harm": 1}),
		screeningFixtureSynthetic("syn-clean-2", "Two people hurt near the old market", true, "publish", map[string]float64{"violence/graphic": 1, "illicit": borderline}),
		screeningFixtureSynthetic("syn-clean-3", "A hateful message sent at the line", true, "reject", map[string]float64{"hate": reject}),
		screeningFixtureSynthetic("syn-clean-4", strings.Repeat("z", service.GibberishRunLength)+" road", false, "reject", map[string]float64{}),
	}
	t.Run("a clean set gives no problem", func(t *testing.T) {
		if got := screeningFixtureProblems(clean); len(got) != 0 {
			t.Errorf("clean set gave problems: %v", got)
		}
		if got := screeningFixtureModelProblems(clean); len(got) != 0 {
			t.Errorf("clean set gave model problems: %v", got)
		}
	})

	failing := []screeningFixtureEntry{
		screeningFixtureSynthetic("syn-fail-1", "Water is rising in the lane", false, "publish", map[string]float64{"harassment": reject}),
		screeningFixtureSynthetic("syn-fail-2", "Please send a boat to the school", true, "publish", map[string]float64{"hate": 1}),
		screeningFixtureSynthetic("syn-fail-3", strings.Repeat("a", service.GibberishRunLength)+" flood near the bridge", false, "publish", map[string]float64{}),
		screeningFixtureSynthetic("syn-fail-4", "An abusive message nobody should post", false, "reject", map[string]float64{"sexual": justBelow}),
		screeningFixtureSynthetic("syn-fail-5", "Another abusive message in a report", true, "reject", map[string]float64{"violence": 1, "self-harm": 1}),
	}
	t.Run("each failure kind produces a problem naming its id", func(t *testing.T) {
		got := screeningFixtureProblems(failing)
		if len(got) != len(failing) {
			t.Fatalf("got %d problems, want %d: %v", len(got), len(failing), got)
		}
		wantWords := []string{"rejected", "withheld", "gibberish", "not rejected", "not rejected"}
		for i, p := range got {
			if !strings.Contains(p, failing[i].ID) {
				t.Errorf("problem %d does not name %s: %s", i, failing[i].ID, p)
			}
			if strings.Contains(p, failing[i].Text) {
				t.Errorf("problem %d prints the entry text", i)
			}
			if !strings.Contains(p, wantWords[i]) {
				t.Errorf("problem %d lacks %q: %s", i, wantWords[i], p)
			}
		}
		if strings.Contains(got[0], "withheld") {
			t.Errorf("a genuine rejection must not be called withheld: %s", got[0])
		}
	})

	t.Run("a wrong model gives exactly one problem", func(t *testing.T) {
		a := screeningFixtureSynthetic("syn-model-1", "Road open now", false, "publish", map[string]float64{"violence": 0})
		a.Model = "omni-moderation-latest"
		b := screeningFixtureSynthetic("syn-model-2", "Road open now again", false, "publish", map[string]float64{"violence": 0})
		got := screeningFixtureModelProblems([]screeningFixtureEntry{a, b})
		if len(got) != 1 || !strings.Contains(got[0], "syn-model-1") {
			t.Errorf("want one problem naming syn-model-1, got %v", got)
		}
	})

	t.Run("the record state counts recorded entries and names the others", func(t *testing.T) {
		recorded := screeningFixtureSynthetic("syn-state-1", "Road open now", false, "publish", map[string]float64{"violence": 0})
		blankA := screeningFixtureEntry{ID: "syn-state-2", Scores: map[string]float64{}}
		blankB := screeningFixtureEntry{ID: "syn-state-3", Scores: map[string]float64{}}
		n, missing := screeningFixtureRecordState([]screeningFixtureEntry{recorded, blankA, blankB})
		if n != 1 || strings.Join(missing, ",") != "syn-state-2,syn-state-3" {
			t.Errorf("got %d recorded and missing %v", n, missing)
		}
	})
}

func TestScreeningFixture(t *testing.T) {
	genuine, abusive := screeningFixtureAll(t)
	all := append(append([]screeningFixtureEntry{}, genuine...), abusive...)
	recorded, missing := screeningFixtureRecordState(all)
	if recorded == 0 {
		t.Skipf("none of the %d fixture entries has recorded scores; record them once with MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord (plan 04-27)", len(all))
	}
	if len(missing) > 0 {
		t.Fatalf("%d of %d fixture entries have no recorded scores, missing ids: %s", len(missing), len(all), strings.Join(missing, ", "))
	}
	if wrong := screeningFixtureModelProblems(all); len(wrong) > 0 {
		t.Fatalf("entries not recorded on the pinned snapshot:\n%s", strings.Join(wrong, "\n"))
	}
	for _, p := range screeningFixtureProblems(all) {
		t.Error(p)
	}

	rejected, borderline := 0, 0
	perRejected := map[string]int{}
	perBorderline := map[string]int{}
	perCount := map[string]int{}
	seriousGenuine, withheld := 0, 0
	var topGenuine float64
	var topID string
	for _, e := range genuine {
		d := screeningFixtureDecision(e)
		perCount[e.Language]++
		if e.Serious {
			seriousGenuine++
		}
		switch d.Verdict {
		case service.VerdictReject:
			rejected++
			perRejected[e.Language]++
			if e.Serious {
				withheld++
			}
		case service.VerdictBorderline:
			borderline++
			perBorderline[e.Language]++
		}
		if s := screeningFixtureMaxRejectScore(e.Scores); s > topGenuine {
			topGenuine, topID = s, e.ID
		}
	}
	t.Logf("genuine: %d entries, %d rejected, %d borderline (posted and listed for review)", len(genuine), rejected, borderline)
	for _, l := range screeningFixtureLanguages {
		t.Logf("genuine %s: %d entries, %d rejected, %d borderline", l, perCount[l], perRejected[l], perBorderline[l])
	}
	t.Logf("serious genuine: %d entries, %d withheld", seriousGenuine, withheld)

	abusiveRejected := 0
	lowAbusive, lowID := float64(2), ""
	for _, e := range abusive {
		d := screeningFixtureDecision(e)
		if d.Verdict == service.VerdictReject {
			abusiveRejected++
		}
		idReason := strings.Split(e.ID, "-")[1]
		if idReason == string(service.RejectGibberish) {
			continue
		}
		if s := screeningFixtureMaxRejectScore(e.Scores); s < lowAbusive {
			lowAbusive, lowID = s, e.ID
		}
		if d.Verdict == service.VerdictReject && string(d.Reason) != idReason {
			t.Logf("%s decided %s but its id says %s (not a failure)", e.ID, d.Reason, idReason)
		}
	}
	t.Logf("abusive: %d of %d rejected", abusiveRejected, len(abusive))
	t.Logf("margin: highest reject score among genuine %.4f (%s), lowest among score based abusive %.4f (%s)", topGenuine, topID, lowAbusive, lowID)
	if rejected == 0 {
		t.Logf("zero of %d genuine entries rejected: the true rate is probably under about 3 in %d (rule of three), not the one in a thousand design target; these texts were written for this test, not taken from real traffic", len(genuine), len(genuine))
	}
}
