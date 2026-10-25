package push

import (
	"encoding/base64"
	"strings"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func TestLoadConfig(t *testing.T) {
	// Throwaway pairs made in memory; nothing here is a real key.
	privA, pubA, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	_, pubB, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	pubRaw, _ := base64.RawURLEncoding.DecodeString(pubA)
	badFirst := append([]byte{0x02}, pubRaw[1:]...)
	privRaw, _ := base64.RawURLEncoding.DecodeString(privA)

	good := func(extra map[string]string) map[string]string {
		env := map[string]string{"VAPID_PUBLIC_KEY": pubA, "VAPID_PRIVATE_KEY": privA, "VAPID_SUBJECT": "ops@example.com"}
		for k, v := range extra {
			if v == "" {
				delete(env, k)
			} else {
				env[k] = v
			}
		}
		return env
	}

	cases := []struct {
		name        string
		env         map[string]string
		wantOK      bool
		wantErr     string // substring of the error, empty for none
		wantSubject string
	}{
		{name: "both keys unset turns push off", env: map[string]string{}, wantOK: false},
		{name: "both keys unset ignores a subject", env: map[string]string{"VAPID_SUBJECT": "nonsense"}, wantOK: false},
		{name: "only public key set", env: good(map[string]string{"VAPID_PRIVATE_KEY": ""}), wantErr: "VAPID_PRIVATE_KEY"},
		{name: "only private key set", env: good(map[string]string{"VAPID_PUBLIC_KEY": ""}), wantErr: "VAPID_PUBLIC_KEY"},
		{name: "public key wrong length", env: good(map[string]string{"VAPID_PUBLIC_KEY": base64.RawURLEncoding.EncodeToString(pubRaw[:64])}), wantErr: "VAPID_PUBLIC_KEY"},
		{name: "public key wrong first byte", env: good(map[string]string{"VAPID_PUBLIC_KEY": base64.RawURLEncoding.EncodeToString(badFirst)}), wantErr: "VAPID_PUBLIC_KEY"},
		{name: "public key not base64", env: good(map[string]string{"VAPID_PUBLIC_KEY": "!!!"}), wantErr: "VAPID_PUBLIC_KEY"},
		{name: "private key wrong length", env: good(map[string]string{"VAPID_PRIVATE_KEY": base64.RawURLEncoding.EncodeToString(privRaw[:31])}), wantErr: "VAPID_PRIVATE_KEY"},
		{name: "private key not base64", env: good(map[string]string{"VAPID_PRIVATE_KEY": "@@@"}), wantErr: "VAPID_PRIVATE_KEY"},
		{name: "keys from two different pairs", env: good(map[string]string{"VAPID_PUBLIC_KEY": pubB}), wantErr: "do not belong together"},
		{name: "good pair with a bare address", env: good(nil), wantOK: true, wantSubject: "ops@example.com"},
		{name: "mailto prefix is stripped", env: good(map[string]string{"VAPID_SUBJECT": "mailto:ops@example.com"}), wantOK: true, wantSubject: "ops@example.com"},
		{name: "https url kept as is", env: good(map[string]string{"VAPID_SUBJECT": "https://pinalert.example"}), wantOK: true, wantSubject: "https://pinalert.example"},
		{name: "surrounding spaces are trimmed", env: good(map[string]string{"VAPID_SUBJECT": "  ops@example.com "}), wantOK: true, wantSubject: "ops@example.com"},
		{name: "http url refused", env: good(map[string]string{"VAPID_SUBJECT": "http://x"}), wantErr: "VAPID_SUBJECT"},
		{name: "not an address refused", env: good(map[string]string{"VAPID_SUBJECT": "not an address"}), wantErr: "VAPID_SUBJECT"},
		{name: "display name refused", env: good(map[string]string{"VAPID_SUBJECT": "Ops <ops@example.com>"}), wantErr: "VAPID_SUBJECT"},
		{name: "https url without host refused", env: good(map[string]string{"VAPID_SUBJECT": "https://"}), wantErr: "VAPID_SUBJECT"},
		{name: "subject falls back to CONTACT_EMAIL", env: good(map[string]string{"VAPID_SUBJECT": "", "CONTACT_EMAIL": "owner@example.com"}), wantOK: true, wantSubject: "owner@example.com"},
		{name: "subject falls back to https BASE_URL", env: good(map[string]string{"VAPID_SUBJECT": "", "BASE_URL": "https://pinalert.example"}), wantOK: true, wantSubject: "https://pinalert.example"},
		{name: "CONTACT_EMAIL beats BASE_URL", env: good(map[string]string{"VAPID_SUBJECT": "", "CONTACT_EMAIL": "owner@example.com", "BASE_URL": "https://pinalert.example"}), wantOK: true, wantSubject: "owner@example.com"},
		{name: "VAPID_SUBJECT beats CONTACT_EMAIL", env: good(map[string]string{"CONTACT_EMAIL": "owner@example.com"}), wantOK: true, wantSubject: "ops@example.com"},
		{name: "http BASE_URL is not a fallback", env: good(map[string]string{"VAPID_SUBJECT": "", "BASE_URL": "http://localhost:3000"}), wantErr: "VAPID_SUBJECT"},
		{name: "no subject anywhere", env: good(map[string]string{"VAPID_SUBJECT": ""}), wantErr: "VAPID_SUBJECT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, ok, err := LoadConfig(func(k string) string { return tc.env[k] })
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				if ok {
					t.Fatal("ok must be false on error")
				}
				for _, secret := range []string{privA, pubA, pubB} {
					if strings.Contains(err.Error(), secret) {
						t.Fatalf("error text echoes a key value: %q", err.Error())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if cfg != (Config{}) {
					t.Fatalf("config must be empty when push is off, got %+v", cfg)
				}
				return
			}
			if cfg.PublicKey != pubA || cfg.PrivateKey != privA || cfg.Subject != tc.wantSubject {
				t.Fatalf("config subject = %q, want %q (keys must round trip)", cfg.Subject, tc.wantSubject)
			}
		})
	}
}
