package service_test

import (
	"maps"
	"math"
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
