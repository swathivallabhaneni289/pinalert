package service_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"pinalert/internal/moderation"
	"pinalert/internal/service"
)

// fakeTextScreener is a scripted service.TextScreener that records what it was
// given. With block set it waits for the context to end, like a stuck provider.
type fakeTextScreener struct {
	mu     sync.Mutex
	result moderation.Result
	err    error
	block  bool

	calls       int
	texts       []string
	deadlines   []time.Duration
	hadDeadline []bool
}

func (f *fakeTextScreener) Screen(ctx context.Context, text string) (moderation.Result, error) {
	f.mu.Lock()
	f.calls++
	f.texts = append(f.texts, text)
	dl, ok := ctx.Deadline()
	f.hadDeadline = append(f.hadDeadline, ok)
	if ok {
		f.deadlines = append(f.deadlines, time.Until(dl))
	} else {
		f.deadlines = append(f.deadlines, 0)
	}
	block, result, err := f.block, f.result, f.err
	f.mu.Unlock()
	if block {
		<-ctx.Done()
		return moderation.Result{}, ctx.Err()
	}
	return result, err
}

func (f *fakeTextScreener) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func screenerWith(scores map[string]float64) *fakeTextScreener {
	return &fakeTextScreener{result: moderation.Result{Model: moderation.DefaultModel, Scores: scores}}
}

// captureLog points the standard logger at a buffer for the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

const p11PlainText = "Water rising near the school gate"

func TestCheckText(t *testing.T) {
	ctx := context.Background()

	t.Run("gibberish_never_calls", func(t *testing.T) {
		fs := screenerWith(map[string]float64{})
		got := service.CheckText(ctx, fs, "   zzzzzzzzzzzzzzzzzzzz   ")
		if got.Outcome != service.TextRejected || got.Reason != service.RejectGibberish {
			t.Fatalf("got %+v, want rejected gibberish", got)
		}
		if fs.callCount() != 0 {
			t.Fatalf("screener called %d times, want 0", fs.callCount())
		}
	})

	t.Run("nil_screener_is_clear", func(t *testing.T) {
		got := service.CheckText(ctx, nil, p11PlainText)
		if got.Outcome != service.TextClear || got.Scores != nil || got.Model != "" {
			t.Fatalf("got %+v, want a bare clear", got)
		}
	})

	t.Run("hateful_rejects", func(t *testing.T) {
		scores := map[string]float64{"hate": service.RejectScoreThreshold}
		fs := screenerWith(scores)
		got := service.CheckText(ctx, fs, p11PlainText)
		if got.Outcome != service.TextRejected || got.Reason != service.RejectHateful {
			t.Fatalf("got %+v, want rejected hateful", got)
		}
		if got.Scores["hate"] != service.RejectScoreThreshold || got.Model != moderation.DefaultModel {
			t.Fatalf("scores or model not carried: %+v", got)
		}
		if fs.callCount() != 1 {
			t.Fatalf("calls %d, want 1", fs.callCount())
		}
	})

	t.Run("harassing_and_sexual", func(t *testing.T) {
		got := service.CheckText(ctx, screenerWith(map[string]float64{"harassment/threatening": service.RejectScoreThreshold}), p11PlainText)
		if got.Outcome != service.TextRejected || got.Reason != service.RejectHarassing {
			t.Fatalf("got %+v, want rejected harassing", got)
		}
		got = service.CheckText(ctx, screenerWith(map[string]float64{"sexual": service.RejectScoreThreshold}), p11PlainText)
		if got.Outcome != service.TextRejected || got.Reason != service.RejectSexual {
			t.Fatalf("got %+v, want rejected sexual", got)
		}
	})

	t.Run("violence_alone_is_borderline", func(t *testing.T) {
		got := service.CheckText(ctx, screenerWith(map[string]float64{"violence": 1.0, "self-harm": 1.0}), p11PlainText)
		if got.Outcome != service.TextBorderline || got.Reason != "" {
			t.Fatalf("got %+v, want borderline with no reason", got)
		}
	})

	t.Run("borderline", func(t *testing.T) {
		got := service.CheckText(ctx, screenerWith(map[string]float64{"hate": service.BorderlineScoreThreshold}), p11PlainText)
		if got.Outcome != service.TextBorderline {
			t.Fatalf("got %+v, want borderline", got)
		}
		if got.Scores["hate"] != service.BorderlineScoreThreshold || got.Model != moderation.DefaultModel {
			t.Fatalf("scores or model not carried: %+v", got)
		}
	})

	t.Run("pass", func(t *testing.T) {
		got := service.CheckText(ctx, screenerWith(map[string]float64{"hate": service.BorderlineScoreThreshold / 2}), p11PlainText)
		if got.Outcome != service.TextClear {
			t.Fatalf("got %+v, want clear", got)
		}
		if got.Scores["hate"] != service.BorderlineScoreThreshold/2 || got.Model != moderation.DefaultModel {
			t.Fatalf("scores or model not carried: %+v", got)
		}
	})

	t.Run("only_trimmed_text_sent", func(t *testing.T) {
		fs := screenerWith(map[string]float64{})
		service.CheckText(ctx, fs, "  Water rising near the school gate \n")
		if len(fs.texts) != 1 || fs.texts[0] != p11PlainText {
			t.Fatalf("screener received %q, want exactly %q", fs.texts, p11PlainText)
		}
	})

	t.Run("one_bounded_attempt", func(t *testing.T) {
		fs := screenerWith(map[string]float64{})
		service.CheckText(ctx, fs, p11PlainText)
		if fs.callCount() != 1 || !fs.hadDeadline[0] {
			t.Fatalf("calls %d, deadline set %v", fs.callCount(), fs.hadDeadline)
		}
		if fs.deadlines[0] <= 0 || fs.deadlines[0] > service.InlineScreenTimeout {
			t.Fatalf("remaining time %v, want in (0, %v]", fs.deadlines[0], service.InlineScreenTimeout)
		}
	})

	t.Run("transient_error_unfinished_once", func(t *testing.T) {
		fs := &fakeTextScreener{err: &moderation.CallError{Status: 429, Transient: true, RetryAfter: 7 * time.Second}}
		got := service.CheckText(ctx, fs, p11PlainText)
		if got.Outcome != service.TextUnfinished || got.RetryAfter != 7*time.Second {
			t.Fatalf("got %+v, want unfinished with 7s", got)
		}
		if fs.callCount() != 1 {
			t.Fatalf("calls %d, want exactly 1 (no inline retry)", fs.callCount())
		}
	})

	t.Run("permanent_error_unfinished", func(t *testing.T) {
		got := service.CheckText(ctx, &fakeTextScreener{err: &moderation.CallError{Status: 401}}, p11PlainText)
		if got.Outcome != service.TextUnfinished || got.RetryAfter != 0 {
			t.Fatalf("got %+v, want unfinished with no wait", got)
		}
		got = service.CheckText(ctx, &fakeTextScreener{err: errors.New("p11 boom")}, p11PlainText)
		if got.Outcome != service.TextUnfinished || got.RetryAfter != 0 {
			t.Fatalf("got %+v, want unfinished with no wait", got)
		}
	})

	t.Run("parent_deadline_respected", func(t *testing.T) {
		parent, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		got := service.CheckText(parent, &fakeTextScreener{block: true}, p11PlainText)
		if got.Outcome != service.TextUnfinished {
			t.Fatalf("got %+v, want unfinished", got)
		}
		if time.Since(start) > time.Second {
			t.Fatalf("took %v, want under 1s", time.Since(start))
		}
	})

	t.Run("log_carries_no_text", func(t *testing.T) {
		buf := captureLog(t)
		fs := &fakeTextScreener{err: &moderation.CallError{Status: 500, Transient: true}}
		service.CheckText(ctx, fs, "p11-secret-marker words here")
		if strings.TrimSpace(buf.String()) == "" {
			t.Fatal("expected at least one log line for an unfinished check")
		}
		if strings.Contains(buf.String(), "p11-secret-marker") {
			t.Fatalf("log carries the text: %s", buf.String())
		}
	})
}
