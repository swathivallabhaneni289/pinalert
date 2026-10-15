package moderation

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

// TestLoadScreener covers the five key and ENV combinations. t.Setenv keeps
// the real OPENAI_API_KEY out of every case, and no case calls Screen on a
// *Client, so nothing here touches the network.
func TestLoadScreener(t *testing.T) {
	t.Run("key set, env production returns the real client", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("MODERATION_MODEL", "")

		s, err := LoadScreener("production")
		if err != nil {
			t.Fatalf("LoadScreener returned error %v, want nil", err)
		}
		c, ok := s.(*Client)
		if !ok {
			t.Fatalf("LoadScreener returned %T, want *Client", s)
		}
		if c.model != "omni-moderation-2024-09-26" {
			t.Errorf("model = %q, want the pinned snapshot", c.model)
		}
	})

	t.Run("key set, env development still returns the real client", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("MODERATION_MODEL", "")

		s, err := LoadScreener("development")
		if err != nil {
			t.Fatalf("LoadScreener returned error %v, want nil", err)
		}
		if _, ok := s.(*Client); !ok {
			t.Fatalf("LoadScreener returned %T, want *Client: a set key wins regardless of env", s)
		}
	})

	t.Run("key unset, env development returns the pass screener and warns", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		t.Setenv("MODERATION_MODEL", "")

		var buf bytes.Buffer
		prevWriter, prevFlags := log.Writer(), log.Flags()
		log.SetOutput(&buf)
		t.Cleanup(func() {
			log.SetOutput(prevWriter)
			log.SetFlags(prevFlags)
		})

		s, err := LoadScreener("development")
		if err != nil {
			t.Fatalf("LoadScreener returned error %v, want nil", err)
		}
		if _, ok := s.(PassScreener); !ok {
			t.Fatalf("LoadScreener returned %T, want PassScreener", s)
		}

		res, err := s.Screen(context.Background(), "anything at all")
		if err != nil {
			t.Fatalf("PassScreener.Screen error %v", err)
		}
		if res.Model != PassModel || res.Scores == nil || len(res.Scores) != 0 {
			t.Errorf("PassScreener result = %+v, want model %q and an empty non nil map", res, PassModel)
		}

		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], "WARNING") || !strings.Contains(lines[0], "OPENAI_API_KEY") {
			t.Errorf("log output = %q, want exactly one WARNING line naming OPENAI_API_KEY", buf.String())
		}
	})

	for _, env := range []string{"", "production"} {
		name := "key unset, env " + env + " returns a nil interface and an error naming OPENAI_API_KEY"
		if env == "" {
			name = "key unset, env empty returns a nil interface and an error naming OPENAI_API_KEY"
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("MODERATION_MODEL", "")

			s, err := LoadScreener(env)
			if err == nil {
				t.Fatal("LoadScreener returned nil error, want an error naming OPENAI_API_KEY")
			}
			if s != nil {
				t.Fatalf("LoadScreener returned non nil Screener %v alongside an error, want a nil interface", s)
			}
			if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
				t.Fatalf("error %q does not name OPENAI_API_KEY", err.Error())
			}
		})
	}
}

func TestLoadScreenerModelOverride(t *testing.T) {
	t.Run("override is used", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("MODERATION_MODEL", "omni-moderation-latest")
		s, err := LoadScreener("production")
		if err != nil {
			t.Fatalf("LoadScreener: %v", err)
		}
		if got := s.(*Client).model; got != "omni-moderation-latest" {
			t.Errorf("model = %q", got)
		}
	})

	t.Run("spaces only model falls back to the default", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "test-key")
		t.Setenv("MODERATION_MODEL", "   ")
		s, err := LoadScreener("production")
		if err != nil {
			t.Fatalf("LoadScreener: %v", err)
		}
		if got := s.(*Client).model; got != DefaultModel {
			t.Errorf("model = %q, want %q", got, DefaultModel)
		}
	})

	t.Run("spaces only key counts as unset", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "   ")
		t.Setenv("MODERATION_MODEL", "")
		s, err := LoadScreener("production")
		if err == nil || s != nil {
			t.Fatalf("LoadScreener = (%v, %v), want a nil interface and an error", s, err)
		}
		if !strings.Contains(err.Error(), "OPENAI_API_KEY") {
			t.Errorf("error %q does not name OPENAI_API_KEY", err.Error())
		}
	})
}
