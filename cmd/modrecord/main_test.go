package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"pinalert/internal/moderation"
)

const pinned = "omni-moderation-2024-09-26"

// fakeScreener answers from a script and records every text it was asked.
type fakeScreener struct {
	texts  []string
	answer func(call int, text string) (moderation.Result, error)
}

func (f *fakeScreener) Screen(ctx context.Context, text string) (moderation.Result, error) {
	f.texts = append(f.texts, text)
	return f.answer(len(f.texts), text)
}

func okAnswer(model string) func(int, string) (moderation.Result, error) {
	return func(int, string) (moderation.Result, error) {
		return moderation.Result{Model: model, Scores: map[string]float64{"hate": 0.25, "violence": 0.5}}, nil
	}
}

// harness wires a fake screener, a factory that records its arguments, a wait
// that records rune counts and a sleep that records durations.
type harness struct {
	fake       *fakeScreener
	factoryHit int
	gotKey     string
	gotModel   string
	runes      []int
	sleeps     []time.Duration
}

func (h *harness) deps() deps {
	return deps{
		newScreener: func(apiKey, model string) moderation.Screener {
			h.factoryHit++
			h.gotKey, h.gotModel = apiKey, model
			return h.fake
		},
		wait:  func(ctx context.Context, runes int) error { h.runes = append(h.runes, runes); return nil },
		sleep: func(d time.Duration) { h.sleeps = append(h.sleeps, d) },
	}
}

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

var liveEnv = map[string]string{"MODERATION_LIVE": "1", "OPENAI_API_KEY": "sk-test-SECRETKEY"}

func writeFixture(t *testing.T, path string, entries []fixtureEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readEntries(t *testing.T, path string) []fixtureEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []fixtureEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func setup(t *testing.T, genuine, abusive []fixtureEntry) (gp, ap string, gRaw, aRaw []byte) {
	t.Helper()
	dir := t.TempDir()
	gp, ap = filepath.Join(dir, "genuine.json"), filepath.Join(dir, "abusive.json")
	return gp, ap, writeFixture(t, gp, genuine), writeFixture(t, ap, abusive)
}

func entry(id, text string) fixtureEntry {
	return fixtureEntry{ID: id, Text: text, Language: "en", Serious: true, Expect: "allow", Scores: map[string]float64{}}
}

func recorded(id, text string) fixtureEntry {
	e := entry(id, text)
	e.Model = pinned
	e.Scores = map[string]float64{"hate": 0.01}
	return e
}

func TestRecorderRefusesWithoutLiveFlag(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"live unset", map[string]string{"OPENAI_API_KEY": "sk-x"}, "MODERATION_LIVE"},
		{"live zero", map[string]string{"MODERATION_LIVE": "0", "OPENAI_API_KEY": "sk-x"}, "MODERATION_LIVE"},
		{"live true", map[string]string{"MODERATION_LIVE": "true", "OPENAI_API_KEY": "sk-x"}, "MODERATION_LIVE"},
		{"key unset", map[string]string{"MODERATION_LIVE": "1"}, "OPENAI_API_KEY"},
		{"key spaces", map[string]string{"MODERATION_LIVE": "1", "OPENAI_API_KEY": "   "}, "OPENAI_API_KEY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gp, ap, gRaw, aRaw := setup(t, []fixtureEntry{entry("g1", "Flood")}, []fixtureEntry{entry("a1", "Abuse")})
			h := &harness{fake: &fakeScreener{answer: okAnswer(pinned)}}
			var out, errb bytes.Buffer
			code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(tc.env), &out, &errb, h.deps())
			if code != 1 {
				t.Fatalf("run = %d, want 1", code)
			}
			if h.factoryHit != 0 || len(h.fake.texts) != 0 {
				t.Errorf("provider side touched: factory %d, screen %d", h.factoryHit, len(h.fake.texts))
			}
			if !strings.Contains(errb.String(), tc.want) {
				t.Errorf("stderr %q does not name %s", errb.String(), tc.want)
			}
			g2, _ := os.ReadFile(gp)
			a2, _ := os.ReadFile(ap)
			if !bytes.Equal(g2, gRaw) || !bytes.Equal(a2, aRaw) {
				t.Errorf("fixture files changed")
			}
		})
	}
}

func TestRecorderFillsModelAndScores(t *testing.T) {
	gp, ap, _, _ := setup(t,
		[]fixtureEntry{entry("g1", "Flood on MG Road"), recorded("g2", "Shelter open")},
		[]fixtureEntry{entry("a1", "Abusive words")})
	h := &harness{fake: &fakeScreener{answer: okAnswer(pinned)}}
	var out, errb bytes.Buffer
	code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(liveEnv), &out, &errb, h.deps())
	if code != 0 {
		t.Fatalf("run = %d, stderr %q", code, errb.String())
	}
	if !reflect.DeepEqual(h.fake.texts, []string{"Flood on MG Road", "Abusive words"}) {
		t.Errorf("screened texts = %v", h.fake.texts)
	}
	g := readEntries(t, gp)
	if g[0].Model != pinned || g[0].Scores["hate"] != 0.25 || g[0].Scores["violence"] != 0.5 {
		t.Errorf("g1 = %+v", g[0])
	}
	if !reflect.DeepEqual(g[1], recorded("g2", "Shelter open")) {
		t.Errorf("already recorded entry changed: %+v", g[1])
	}
	a := readEntries(t, ap)
	if a[0].Model != pinned || len(a[0].Scores) != 2 {
		t.Errorf("a1 = %+v", a[0])
	}
	if strings.Contains(out.String(), "Flood on MG Road") || strings.Contains(out.String(), "Abusive words") {
		t.Errorf("stdout carries a text: %q", out.String())
	}
	if !strings.Contains(out.String(), "recorded g1") || !strings.Contains(out.String(), "recorded a1") {
		t.Errorf("stdout lacks ids: %q", out.String())
	}
	if strings.Contains(out.String()+errb.String(), "SECRETKEY") {
		t.Errorf("output carries the key")
	}
}

func TestRecorderKeepsTextByteIdentical(t *testing.T) {
	texts := []string{
		"Water & mud <knee deep> near the bridge",
		"మా వీధిలో వరద నీరు వచ్చింది",
		"हमारे इलाके में पानी भर गया है",
		"Bhai pani bahut aa gaya, help karo yaar",
		"HELP WE ARE TRAPPED ON THE ROOF",
		"Call 9876543210 for the boat",
	}
	var es []fixtureEntry
	for i, tx := range texts {
		es = append(es, entry("g"+string(rune('1'+i)), tx))
	}
	gp, ap, _, _ := setup(t, es, []fixtureEntry{})
	h := &harness{fake: &fakeScreener{answer: okAnswer(pinned)}}
	var out, errb bytes.Buffer
	if code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(liveEnv), &out, &errb, h.deps()); code != 0 {
		t.Fatalf("run = %d, stderr %q", code, errb.String())
	}
	got := readEntries(t, gp)
	for i, tx := range texts {
		if got[i].Text != tx {
			t.Errorf("text %d changed: %q", i, got[i].Text)
		}
	}
	raw, _ := os.ReadFile(gp)
	if !bytes.Contains(raw, []byte("Water & mud <knee deep>")) {
		t.Errorf("raw file lost the literal & and <")
	}
	if !bytes.HasSuffix(raw, []byte("]\n")) {
		t.Errorf("file does not end with a newline after the array")
	}
}

func TestRecorderAlwaysUsesPinnedModel(t *testing.T) {
	env := map[string]string{"MODERATION_LIVE": "1", "OPENAI_API_KEY": "sk-x", "MODERATION_MODEL": "omni-moderation-latest"}
	gp, ap, _, _ := setup(t, []fixtureEntry{entry("g1", "Flood")}, []fixtureEntry{})
	h := &harness{fake: &fakeScreener{answer: okAnswer(pinned)}}
	var out, errb bytes.Buffer
	if code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(env), &out, &errb, h.deps()); code != 0 {
		t.Fatalf("run = %d, stderr %q", code, errb.String())
	}
	if h.gotModel != pinned {
		t.Errorf("factory model = %q, want %q", h.gotModel, pinned)
	}

	gp2, ap2, gRaw, _ := setup(t, []fixtureEntry{entry("g1", "Flood")}, []fixtureEntry{})
	h2 := &harness{fake: &fakeScreener{answer: okAnswer("omni-moderation-latest")}}
	out.Reset()
	errb.Reset()
	if code := run([]string{"-genuine", gp2, "-abusive", ap2}, lookupFrom(liveEnv), &out, &errb, h2.deps()); code != 1 {
		t.Fatalf("mismatched model: run = %d, want 1", code)
	}
	g2, _ := os.ReadFile(gp2)
	if !bytes.Equal(g2, gRaw) {
		t.Errorf("entry was recorded from a mismatched model")
	}
}

func TestRecorderStopsOnQuota(t *testing.T) {
	gp, ap, gRaw, aRaw := setup(t, []fixtureEntry{entry("g1", "Flood"), entry("g2", "Road")}, []fixtureEntry{entry("a1", "Bad")})
	h := &harness{fake: &fakeScreener{answer: func(int, string) (moderation.Result, error) {
		return moderation.Result{}, &moderation.CallError{Status: 429, Quota: true}
	}}}
	var out, errb bytes.Buffer
	if code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(liveEnv), &out, &errb, h.deps()); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if len(h.fake.texts) != 1 {
		t.Errorf("screen calls = %d, want 1", len(h.fake.texts))
	}
	if !strings.Contains(errb.String(), "insufficient_quota") {
		t.Errorf("stderr %q does not name insufficient_quota", errb.String())
	}
	g2, _ := os.ReadFile(gp)
	a2, _ := os.ReadFile(ap)
	if !bytes.Equal(g2, gRaw) || !bytes.Equal(a2, aRaw) {
		t.Errorf("fixture files changed")
	}
}

func TestRecorderRetriesTransientThenStops(t *testing.T) {
	gp, ap, _, _ := setup(t, []fixtureEntry{entry("g-flaky", "Flood")}, []fixtureEntry{})
	h := &harness{fake: &fakeScreener{answer: func(int, string) (moderation.Result, error) {
		return moderation.Result{}, &moderation.CallError{Status: 503, Transient: true, RetryAfter: 45 * time.Second}
	}}}
	var out, errb bytes.Buffer
	if code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(liveEnv), &out, &errb, h.deps()); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if len(h.fake.texts) != 3 {
		t.Errorf("screen calls = %d, want 3", len(h.fake.texts))
	}
	if !reflect.DeepEqual(h.sleeps, []time.Duration{45 * time.Second, 45 * time.Second}) {
		t.Errorf("sleeps = %v", h.sleeps)
	}
	if !strings.Contains(errb.String(), "g-flaky") {
		t.Errorf("stderr %q does not name the entry id", errb.String())
	}

	h2 := &harness{fake: &fakeScreener{answer: func(int, string) (moderation.Result, error) {
		return moderation.Result{}, &moderation.CallError{Status: 500, Transient: true}
	}}}
	h2.fake.answer = func(call int, _ string) (moderation.Result, error) {
		if call == 1 {
			return moderation.Result{}, &moderation.CallError{Status: 500, Transient: true}
		}
		return okAnswer(pinned)(call, "")
	}
	gp2, ap2, _, _ := setup(t, []fixtureEntry{entry("g1", "Flood")}, []fixtureEntry{})
	if code := run([]string{"-genuine", gp2, "-abusive", ap2}, lookupFrom(liveEnv), &out, &errb, h2.deps()); code != 0 {
		t.Fatalf("recovering run = %d", code)
	}
	if !reflect.DeepEqual(h2.sleeps, []time.Duration{20 * time.Second}) {
		t.Errorf("sleeps = %v, want one 20s wait", h2.sleeps)
	}
}

func TestRecorderRejectsUnknownField(t *testing.T) {
	gp, ap, _, aRaw := setup(t, []fixtureEntry{}, []fixtureEntry{entry("a1", "Bad")})
	raw := `[{"id":"g1","text":"Flood","language":"en","serious":true,"expect":"allow","model":"","scores":{},"note":"x"}]`
	if err := os.WriteFile(gp, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &harness{fake: &fakeScreener{answer: okAnswer(pinned)}}
	var out, errb bytes.Buffer
	if code := run([]string{"-genuine", gp, "-abusive", ap}, lookupFrom(liveEnv), &out, &errb, h.deps()); code != 1 {
		t.Fatalf("run = %d, want 1", code)
	}
	if len(h.fake.texts) != 0 {
		t.Errorf("screener was called %d times before the files were validated", len(h.fake.texts))
	}
	g2, _ := os.ReadFile(gp)
	a2, _ := os.ReadFile(ap)
	if string(g2) != raw || !bytes.Equal(a2, aRaw) {
		t.Errorf("files changed")
	}
}
