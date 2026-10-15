package moderation

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// PassModel is the model name a development pass result carries. It can never
// be mistaken for a real provider snapshot.
const PassModel = "development-pass"

// PassScreener approves everything locally. It holds no HTTP client and never
// touches the network, so local development and CI run without a key.
type PassScreener struct{}

// Screen returns an empty score map, which the policy reads as clean.
func (PassScreener) Screen(ctx context.Context, text string) (Result, error) {
	return Result{Model: PassModel, Scores: map[string]float64{}}, nil
}

var _ Screener = PassScreener{}

// LoadScreener selects a Screener from the process environment, following the
// mailer.LoadMailer branch order:
//
//   - OPENAI_API_KEY set: return the real Client, regardless of env. The model
//     comes from MODERATION_MODEL and defaults to DefaultModel.
//   - OPENAI_API_KEY unset and env == "development": return PassScreener and
//     log one WARNING, so local development and CI never touch the network.
//   - OPENAI_API_KEY unset and env is anything else (including ""): return a
//     nil interface and an error naming OPENAI_API_KEY.
//
// LoadScreener never calls log.Fatal; main turns the error into log.Fatalf
// (04-11). The key is never logged.
func LoadScreener(env string) (Screener, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey != "" {
		model := strings.TrimSpace(os.Getenv("MODERATION_MODEL"))
		if model == "" {
			model = DefaultModel
		}
		return NewClient(apiKey, model), nil
	}

	if env != "development" {
		return nil, fmt.Errorf("moderation: OPENAI_API_KEY must be set outside ENV=development")
	}

	log.Println("WARNING: OPENAI_API_KEY unset; text checks pass everything locally and nothing is sent to the provider")
	return PassScreener{}, nil
}
