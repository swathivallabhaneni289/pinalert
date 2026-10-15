package service_test

import (
	"maps"
	"math"
	"strings"
	"testing"

	"pinalert/internal/service"
)

// screeningProviderKeys are the 13 category keys the moderation provider
// documents, in the documented order.
var screeningProviderKeys = []string{
	"harassment",
	"harassment/threatening",
	"hate",
	"hate/threatening",
	"illicit",
	"illicit/violent",
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"sexual",
	"sexual/minors",
	"violence",
	"violence/graphic",
}

// screeningRejectKeys are the six keys that can reject, with the reason each
// one names.
var screeningRejectKeys = []struct {
	key    string
	reason service.RejectReason
}{
	{"hate", service.RejectHateful},
	{"hate/threatening", service.RejectHateful},
	{"harassment", service.RejectHarassing},
	{"harassment/threatening", service.RejectHarassing},
	{"sexual", service.RejectSexual},
	{"sexual/minors", service.RejectSexual},
}

// screeningNonRejectKeys are the seven keys that never reject.
var screeningNonRejectKeys = []string{
	"illicit",
	"illicit/violent",
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"violence",
	"violence/graphic",
}

type screeningDecideCase struct {
	name   string
	scores map[string]float64
	want   service.ScreenDecision
}

func screeningPass() service.ScreenDecision {
	return service.ScreenDecision{Verdict: service.VerdictPass}
}

func screeningBorderline() service.ScreenDecision {
	return service.ScreenDecision{Verdict: service.VerdictBorderline}
}

func screeningReject(r service.RejectReason) service.ScreenDecision {
	return service.ScreenDecision{Verdict: service.VerdictReject, Reason: r}
}

func screeningAll(v float64) map[string]float64 {
	m := make(map[string]float64, len(screeningProviderKeys))
	for _, k := range screeningProviderKeys {
		m[k] = v
	}
	return m
}

func screeningDecideCases() []screeningDecideCase {
	r := service.RejectScoreThreshold
	b := service.BorderlineScoreThreshold
	belowR := math.Nextafter(service.RejectScoreThreshold, 0)
	belowB := math.Nextafter(service.BorderlineScoreThreshold, 0)

	cases := []screeningDecideCase{
		{"a nil score map passes", nil, screeningPass()},
		{"an empty score map passes", map[string]float64{}, screeningPass()},
		{"all 13 keys at zero pass", screeningAll(0), screeningPass()},
		{"all 13 keys just under the borderline line pass", screeningAll(belowB), screeningPass()},
	}

	for _, rk := range screeningRejectKeys {
		cases = append(cases,
			screeningDecideCase{rk.key + " alone at the reject line rejects as " + string(rk.reason),
				map[string]float64{rk.key: r}, screeningReject(rk.reason)},
			screeningDecideCase{rk.key + " alone at 1.0 rejects as " + string(rk.reason),
				map[string]float64{rk.key: 1.0}, screeningReject(rk.reason)},
			screeningDecideCase{rk.key + " alone just under the reject line is borderline",
				map[string]float64{rk.key: belowR}, screeningBorderline()},
			screeningDecideCase{rk.key + " alone at the borderline line is borderline",
				map[string]float64{rk.key: b}, screeningBorderline()},
		)
	}

	for _, k := range screeningNonRejectKeys {
		cases = append(cases,
			screeningDecideCase{k + " alone at 1.0 is borderline and never a reject",
				map[string]float64{k: 1.0}, screeningBorderline()},
			screeningDecideCase{k + " alone at the borderline line is borderline",
				map[string]float64{k: b}, screeningBorderline()},
			screeningDecideCase{k + " alone just under the borderline line passes",
				map[string]float64{k: belowB}, screeningPass()},
		)
	}

	allSeven := make(map[string]float64, len(screeningNonRejectKeys))
	for _, k := range screeningNonRejectKeys {
		allSeven[k] = 1.0
	}
	withHarassment := maps.Clone(allSeven)
	withHarassment["harassment"] = r

	cases = append(cases,
		screeningDecideCase{"all seven non reject keys at 1.0 together are borderline",
			allSeven, screeningBorderline()},
		screeningDecideCase{"an unknown key at 1.0 is borderline",
			map[string]float64{"future/new": 1.0}, screeningBorderline()},
		screeningDecideCase{"an unknown key just under the borderline line passes",
			map[string]float64{"future/new": belowB}, screeningPass()},
		screeningDecideCase{"hate at the reject line and harassment at 1.0 name harassing",
			map[string]float64{"hate": r, "harassment": 1.0}, screeningReject(service.RejectHarassing)},
		screeningDecideCase{"hate at 1.0 and harassment at the reject line name hateful",
			map[string]float64{"hate": 1.0, "harassment": r}, screeningReject(service.RejectHateful)},
		screeningDecideCase{"hate, harassment and sexual all at the reject line tie to hateful",
			map[string]float64{"hate": r, "harassment": r, "sexual": r}, screeningReject(service.RejectHateful)},
		screeningDecideCase{"harassment and sexual both at the reject line tie to harassing",
			map[string]float64{"harassment": r, "sexual": r}, screeningReject(service.RejectHarassing)},
		screeningDecideCase{"hate threatening at 1.0 with hate at zero is hateful",
			map[string]float64{"hate/threatening": 1.0, "hate": 0}, screeningReject(service.RejectHateful)},
		screeningDecideCase{"sexual minors at the reject line is sexual",
			map[string]float64{"sexual/minors": r}, screeningReject(service.RejectSexual)},
		screeningDecideCase{"harassment at the reject line outranks seven non reject keys at 1.0",
			withHarassment, screeningReject(service.RejectHarassing)},
	)
	return cases
}

func TestDecide(t *testing.T) {
	for _, tc := range screeningDecideCases() {
		t.Run(tc.name, func(t *testing.T) {
			before := maps.Clone(tc.scores)
			got := service.Decide(tc.scores, false)
			if got != tc.want {
				t.Fatalf("Decide = %+v, want %+v", got, tc.want)
			}
			if !got.Verdict.Valid() {
				t.Errorf("verdict %q is not valid", got.Verdict)
			}
			if got.Verdict != service.VerdictReject && got.Reason != "" {
				t.Errorf("reason %q set on a %q verdict", got.Reason, got.Verdict)
			}
			if got.Verdict == service.VerdictReject && !got.Reason.Valid() {
				t.Errorf("reject reason %q is not valid", got.Reason)
			}
			if got.Reason == service.RejectGibberish {
				t.Errorf("Decide returned the gibberish reason")
			}
			if !maps.Equal(before, tc.scores) {
				t.Errorf("Decide changed the score map: before %v, after %v", before, tc.scores)
			}
			for i := 0; i < 20; i++ {
				if again := service.Decide(tc.scores, false); again != got {
					t.Fatalf("call %d gave %+v, first call gave %+v", i, again, got)
				}
			}
		})
	}
}

func TestDecideIgnoresSeverity(t *testing.T) {
	for _, tc := range screeningDecideCases() {
		t.Run(tc.name, func(t *testing.T) {
			serious := service.Decide(tc.scores, true)
			plain := service.Decide(tc.scores, false)
			if serious != plain {
				t.Fatalf("serious true gave %+v, serious false gave %+v", serious, plain)
			}
		})
	}

	t.Run("hate at 1.0 with serious true is a hateful reject", func(t *testing.T) {
		got := service.Decide(map[string]float64{"hate": 1.0}, true)
		if got != screeningReject(service.RejectHateful) {
			t.Fatalf("Decide = %+v, want a hateful reject", got)
		}
	})
}

func TestDecideThresholdsOrdered(t *testing.T) {
	if !(0 < service.BorderlineScoreThreshold) {
		t.Errorf("borderline threshold %v must be above 0", service.BorderlineScoreThreshold)
	}
	if !(service.BorderlineScoreThreshold < service.RejectScoreThreshold) {
		t.Errorf("borderline threshold %v must be below reject threshold %v",
			service.BorderlineScoreThreshold, service.RejectScoreThreshold)
	}
	if !(service.RejectScoreThreshold <= 1) {
		t.Errorf("reject threshold %v must be at most 1", service.RejectScoreThreshold)
	}
}

func TestScreeningEnumsAreClosed(t *testing.T) {
	wantVerdicts := []service.Verdict{service.VerdictPass, service.VerdictBorderline, service.VerdictReject}
	wantVerdictSlugs := []string{"pass", "borderline", "reject"}
	if len(service.Verdicts) != len(wantVerdicts) {
		t.Fatalf("Verdicts has %d entries, want %d", len(service.Verdicts), len(wantVerdicts))
	}
	for i, v := range service.Verdicts {
		if v != wantVerdicts[i] || string(v) != wantVerdictSlugs[i] {
			t.Errorf("Verdicts[%d] = %q, want %q", i, v, wantVerdictSlugs[i])
		}
		if !v.Valid() {
			t.Errorf("verdict %q is not valid", v)
		}
	}
	for _, bad := range []service.Verdict{"", "Reject", "PASS", "Borderline"} {
		if bad.Valid() {
			t.Errorf("verdict %q must not be valid", bad)
		}
	}

	wantReasons := []service.RejectReason{service.RejectHateful, service.RejectHarassing, service.RejectSexual, service.RejectGibberish}
	wantReasonSlugs := []string{"hateful", "harassing", "sexual", "gibberish"}
	if len(service.RejectReasons) != len(wantReasons) {
		t.Fatalf("RejectReasons has %d entries, want %d", len(service.RejectReasons), len(wantReasons))
	}
	for i, r := range service.RejectReasons {
		if r != wantReasons[i] || string(r) != wantReasonSlugs[i] {
			t.Errorf("RejectReasons[%d] = %q, want %q", i, r, wantReasonSlugs[i])
		}
		if !r.Valid() {
			t.Errorf("reject reason %q is not valid", r)
		}
	}
	for _, bad := range []service.RejectReason{"", "Hateful", "spam", "SEXUAL"} {
		if bad.Valid() {
			t.Errorf("reject reason %q must not be valid", bad)
		}
	}
}

func TestGibberish(t *testing.T) {
	n := service.GibberishRunLength
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"one letter repeated the run length is gibberish", strings.Repeat("a", n), true},
		{"a letter run inside real words is gibberish", "Road " + strings.Repeat("a", n) + " flooded near bridge", true},
		{"a longer run of capitals is gibberish", strings.Repeat("Z", n+5), true},
		{"a run of capital letters at the run length is gibberish", strings.Repeat("A", n), true},
		{"one word only is gibberish", "qwertyuiop!!", true},
		{"single letters with no two letter word are gibberish", "a b c d e f g h", true},
		{"digits only have no words", "1234567890 0987654321", true},
		{"exclamation marks only have no words", strings.Repeat("!", 12), true},
		{"the empty text has no words", "", true},
		{"spaces only have no words", "   ", true},

		{"a plain sentence is fine", "Road flooded near the bus stand", false},
		{"exactly two words of two letters is fine", "Help me", false},
		{"one short of a run is fine", strings.Repeat("a", n-1) + " flood road", false},
		{"punctuation never forms a run", "HELP" + strings.Repeat("!", n+2) + " water entering house", false},
		{"dots never form a run", "Need boats" + strings.Repeat(".", n+2) + " 3 families on roof", false},
		{"long runs of spaces never form a run", "Road flooded" + strings.Repeat(" ", n+2) + "near bridge", false},
		{"repeated emoji never form a run", "Please help " + strings.Repeat("\U0001F64F", n), false},
		{"a phone number is fine", "Water rising, call 9999999999 now", false},
		{"capitals are fine", "ROAD BLOCKED AT NH44 NEAR TOLL PLAZA", false},
		{"links are fine", "Shelter open, see https://example.org/shelter for the list", false},
		{"Hinglish is fine", "Paani bhar gaya hai, help chahiye", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.IsGibberish(tc.text); got != tc.want {
				t.Errorf("IsGibberish(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestGibberishCountsRunes(t *testing.T) {
	n := service.GibberishRunLength
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"nine Telugu letters are not a run though the bytes exceed the run length",
			strings.Repeat("అ", n-1) + " వరద వచ్చింది", false},
		{"ten Telugu letters are a run",
			strings.Repeat("అ", n) + " వరద వచ్చింది", true},
		{"single Telugu letters are not two letter words",
			"క గ చ ట ప", true},
		{"Hindi vowel signs and nukta stay inside the word", "बाढ़ आई", false},
		{"Tamil vowel sign and virama stay inside the word", "வெள்ளம் வந்தது", false},
		{"Telugu vowel sign, anusvara and virama stay inside the word", "వరద వచ్చింది", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.IsGibberish(tc.text); got != tc.want {
				t.Errorf("IsGibberish(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
