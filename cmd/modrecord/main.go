// Command modrecord records provider scores into the screening fixture files
// (D-07), so the fixture test runs offline against scores captured once from
// the pinned snapshot.
//
// Usage:
//
//	MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord
//	MODERATION_LIVE=1 OPENAI_API_KEY=... go run ./cmd/modrecord -genuine PATH -abusive PATH
//
// It never runs in CI. It always asks for the pinned snapshot
// moderation.DefaultModel and ignores the MODERATION_MODEL override. It
// records only entries whose scores are empty or whose model differs from the
// snapshot, so a run stopped by the daily limit or by quota resumes where it
// stopped. Nothing it prints contains a text or the key.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"pinalert/internal/moderation"
)

const (
	// recorderRequestsPerMinute and recorderRunesPerMinute pace the run under
	// the client's own refill rates, so the client budget never refuses.
	recorderRequestsPerMinute = 150
	recorderRunesPerMinute    = 6000
	recorderRuneBurst         = 1000
	// recorderMaxRetries is how many times one entry is retried after a
	// transient failure.
	recorderMaxRetries = 2
	// recorderRetryWait is the shortest wait before a retry.
	recorderRetryWait = 20 * time.Second
)

// fixtureEntry is one fixture row. The field order is the file layout.
type fixtureEntry struct {
	ID       string             `json:"id"`
	Text     string             `json:"text"`
	Language string             `json:"language"`
	Serious  bool               `json:"serious"`
	Expect   string             `json:"expect"`
	Model    string             `json:"model"`
	Scores   map[string]float64 `json:"scores"`
}

// deps holds everything run needs from outside, so tests use a fake screener.
type deps struct {
	newScreener func(apiKey, model string) moderation.Screener
	wait        func(ctx context.Context, runes int) error
	sleep       func(time.Duration)
}

func defaultDeps() deps {
	requests := rate.NewLimiter(rate.Limit(float64(recorderRequestsPerMinute)/60), 1)
	runes := rate.NewLimiter(rate.Limit(float64(recorderRunesPerMinute)/60), recorderRuneBurst)
	return deps{
		newScreener: func(apiKey, model string) moderation.Screener {
			return moderation.NewClient(apiKey, model)
		},
		wait: func(ctx context.Context, n int) error {
			if err := requests.WaitN(ctx, 1); err != nil {
				return err
			}
			return runes.WaitN(ctx, n)
		},
		sleep: time.Sleep,
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.LookupEnv, os.Stdout, os.Stderr, defaultDeps()))
}

// loadFile decodes one fixture file strictly: no unknown field, no trailing data.
func loadFile(path string) ([]fixtureEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var entries []fixtureEntry
	if err := dec.Decode(&entries); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the array")
	}
	return entries, nil
}

// writeFile replaces path atomically with the entries in the fixture layout.
func writeFile(path string, entries []fixtureEntry) error {
	for i := range entries {
		if entries[i].Scores == nil {
			entries[i].Scores = map[string]float64{}
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".modrecord-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func runeCount(text string) int {
	n := utf8.RuneCountInString(strings.TrimSpace(text))
	if n > 1000 {
		n = 1000
	}
	if n < 1 {
		n = 1
	}
	return n
}

func run(args []string, lookup func(string) (string, bool), stdout, stderr io.Writer, d deps) int {
	fs2 := flag.NewFlagSet("modrecord", flag.ContinueOnError)
	fs2.SetOutput(stderr)
	genuinePath := fs2.String("genuine", "internal/service/testdata/screening/genuine.json", "path of the genuine fixture file")
	abusivePath := fs2.String("abusive", "internal/service/testdata/screening/abusive.json", "path of the abusive fixture file")
	if err := fs2.Parse(args); err != nil {
		return 2
	}

	if v, _ := lookup("MODERATION_LIVE"); v != "1" {
		fmt.Fprintln(stderr, "modrecord: refusing to call the provider: set MODERATION_LIVE=1 to record fixture scores")
		return 1
	}
	key, _ := lookup("OPENAI_API_KEY")
	key = strings.TrimSpace(key)
	if key == "" {
		fmt.Fprintln(stderr, "modrecord: OPENAI_API_KEY must be set")
		return 1
	}

	paths := []string{*genuinePath, *abusivePath}
	files := make([][]fixtureEntry, len(paths))
	for i, p := range paths {
		entries, err := loadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(stderr, "modrecord: %s: not found; the fixture files come from plan 04-28\n", p)
			return 1
		}
		if err != nil {
			fmt.Fprintf(stderr, "modrecord: %s: %v\n", p, err)
			return 1
		}
		files[i] = entries
	}

	screener := d.newScreener(key, moderation.DefaultModel)
	ctx := context.Background()
	done, skipped := 0, 0
	for i, entries := range files {
		for j := range entries {
			e := &entries[j]
			if len(e.Scores) > 0 && e.Model == moderation.DefaultModel {
				skipped++
				continue
			}
			var res moderation.Result
			for attempt := 0; ; attempt++ {
				if err := d.wait(ctx, runeCount(e.Text)); err != nil {
					fmt.Fprintf(stderr, "modrecord: entry %s: %v\n", e.ID, err)
					return 1
				}
				var err error
				res, err = screener.Screen(ctx, e.Text)
				if err == nil {
					break
				}
				var ce *moderation.CallError
				if errors.As(err, &ce) && ce.Quota {
					fmt.Fprintf(stderr, "modrecord: the provider answered insufficient_quota after recording %d entries; this key has no usable quota\n", done)
					return 1
				}
				if errors.As(err, &ce) && ce.Transient {
					if attempt >= recorderMaxRetries {
						fmt.Fprintf(stderr, "modrecord: entry %s could not be checked after %d attempts\n", e.ID, recorderMaxRetries+1)
						return 1
					}
					d.sleep(max(ce.RetryAfter, recorderRetryWait))
					continue
				}
				fmt.Fprintf(stderr, "modrecord: entry %s: %v\n", e.ID, err)
				return 1
			}
			if res.Model != moderation.DefaultModel {
				fmt.Fprintf(stderr, "modrecord: the provider answered with model %s, want %s; nothing recorded for %s\n", res.Model, moderation.DefaultModel, e.ID)
				return 1
			}
			e.Model = res.Model
			e.Scores = res.Scores
			if err := writeFile(paths[i], entries); err != nil {
				fmt.Fprintf(stderr, "modrecord: %s: %v\n", paths[i], err)
				return 1
			}
			done++
			fmt.Fprintf(stdout, "recorded %s\n", e.ID)
		}
	}
	fmt.Fprintf(stdout, "modrecord: recorded %d entries, %d already recorded\n", done, skipped)
	return 0
}
