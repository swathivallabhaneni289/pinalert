package push

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestEndpointPolicy(t *testing.T) {
	long := "https://fcm.googleapis.com/fcm/send/" + strings.Repeat("a", 2100)
	cases := []struct {
		name     string
		endpoint string
		wantHost string
		wantErr  error
	}{
		{name: "fcm", endpoint: "https://fcm.googleapis.com/fcm/send/abc", wantHost: "fcm.googleapis.com"},
		{name: "mozilla", endpoint: "https://updates.push.services.mozilla.com/wpush/v2/abc", wantHost: "updates.push.services.mozilla.com"},
		{name: "apple web", endpoint: "https://web.push.apple.com/abc", wantHost: "web.push.apple.com"},
		{name: "apple api", endpoint: "https://api.push.apple.com/abc", wantHost: "api.push.apple.com"},
		{name: "apple bare suffix", endpoint: "https://push.apple.com/abc", wantHost: "push.apple.com"},
		{name: "windows", endpoint: "https://wns2-par02p.notify.windows.com/w/?token=abc", wantHost: "wns2-par02p.notify.windows.com"},
		{name: "upper case host is lower cased", endpoint: "https://FCM.GoogleAPIs.com/fcm/send/abc", wantHost: "fcm.googleapis.com"},
		{name: "explicit port 443", endpoint: "https://fcm.googleapis.com:443/fcm/send/abc", wantHost: "fcm.googleapis.com"},
		{name: "http scheme", endpoint: "http://fcm.googleapis.com/fcm/send/abc", wantErr: ErrBadEndpoint},
		{name: "ip literal v4", endpoint: "https://203.0.113.9/abc", wantErr: ErrBadEndpoint},
		{name: "ip literal v6", endpoint: "https://[2001:db8::1]/abc", wantErr: ErrBadEndpoint},
		{name: "loopback literal", endpoint: "https://127.0.0.1/abc", wantErr: ErrBadEndpoint},
		{name: "userinfo", endpoint: "https://user:pw@fcm.googleapis.com/abc", wantErr: ErrBadEndpoint},
		{name: "user only", endpoint: "https://user@fcm.googleapis.com/abc", wantErr: ErrBadEndpoint},
		{name: "port 8443", endpoint: "https://fcm.googleapis.com:8443/abc", wantErr: ErrBadEndpoint},
		{name: "lookalike of windows host", endpoint: "https://evil-notify.windows.com/abc", wantErr: ErrUnsupportedService},
		{name: "windows name inside another domain", endpoint: "https://notify.windows.com.evil.example/abc", wantErr: ErrUnsupportedService},
		{name: "apple as a prefix of another name", endpoint: "https://push.apple.com.attacker.example/abc", wantErr: ErrUnsupportedService},
		{name: "unknown but well formed", endpoint: "https://push.example.org/abc", wantErr: ErrUnsupportedService},
		{name: "empty", endpoint: "", wantErr: ErrBadEndpoint},
		{name: "relative", endpoint: "/fcm/send/abc", wantErr: ErrBadEndpoint},
		{name: "no host", endpoint: "https:///abc", wantErr: ErrBadEndpoint},
		{name: "garbage", endpoint: "ht!tp://%zz", wantErr: ErrBadEndpoint},
		{name: "too long", endpoint: long, wantErr: ErrBadEndpoint},
	}
	policy := DefaultEndpointPolicy()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, err := policy.Check(tc.endpoint)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Check error = %v, want %v", err, tc.wantErr)
				}
				if host != "" {
					t.Fatalf("host = %q on error, want empty", host)
				}
				if err != nil && tc.endpoint != "" && strings.Contains(err.Error(), "abc") {
					t.Fatalf("error text carries the endpoint: %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("Check error = %v, want nil", err)
			}
			if host != tc.wantHost {
				t.Fatalf("host = %q, want %q", host, tc.wantHost)
			}
		})
	}
}

func browserKeys(t *testing.T) (p256dh, auth []byte) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth = make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return priv.PublicKey().Bytes(), auth
}

func TestCheckKeys(t *testing.T) {
	pub, auth := browserKeys(t)
	badFirst := append([]byte{0x03}, pub[1:]...)
	cases := []struct {
		name    string
		p256dh  string
		auth    string
		wantErr bool
	}{
		{name: "raw url encoding", p256dh: base64.RawURLEncoding.EncodeToString(pub), auth: base64.RawURLEncoding.EncodeToString(auth)},
		{name: "padded url encoding", p256dh: base64.URLEncoding.EncodeToString(pub), auth: base64.URLEncoding.EncodeToString(auth)},
		{name: "standard encoding", p256dh: base64.StdEncoding.EncodeToString(pub), auth: base64.StdEncoding.EncodeToString(auth)},
		{name: "raw standard encoding", p256dh: base64.RawStdEncoding.EncodeToString(pub), auth: base64.RawStdEncoding.EncodeToString(auth)},
		{name: "short p256dh", p256dh: base64.RawURLEncoding.EncodeToString(pub[:64]), auth: base64.RawURLEncoding.EncodeToString(auth), wantErr: true},
		{name: "long p256dh", p256dh: base64.RawURLEncoding.EncodeToString(append(append([]byte{}, pub...), 1)), auth: base64.RawURLEncoding.EncodeToString(auth), wantErr: true},
		{name: "wrong first byte", p256dh: base64.RawURLEncoding.EncodeToString(badFirst), auth: base64.RawURLEncoding.EncodeToString(auth), wantErr: true},
		{name: "short auth", p256dh: base64.RawURLEncoding.EncodeToString(pub), auth: base64.RawURLEncoding.EncodeToString(auth[:15]), wantErr: true},
		{name: "long auth", p256dh: base64.RawURLEncoding.EncodeToString(pub), auth: base64.RawURLEncoding.EncodeToString(append(append([]byte{}, auth...), 1)), wantErr: true},
		{name: "undecodable p256dh", p256dh: "!!!not base64!!!", auth: base64.RawURLEncoding.EncodeToString(auth), wantErr: true},
		{name: "undecodable auth", p256dh: base64.RawURLEncoding.EncodeToString(pub), auth: "@@@", wantErr: true},
		{name: "empty both", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckKeys(tc.p256dh, tc.auth)
			if tc.wantErr {
				if !errors.Is(err, ErrBadKeys) {
					t.Fatalf("CheckKeys error = %v, want ErrBadKeys", err)
				}
				if tc.p256dh != "" && strings.Contains(err.Error(), tc.p256dh) {
					t.Fatalf("error text echoes a key: %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckKeys error = %v, want nil", err)
			}
		})
	}
}

func TestIsAppleHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"push.apple.com", true},
		{"web.push.apple.com", true},
		{"api.push.apple.com", true},
		{"WEB.PUSH.APPLE.COM", true},
		{"evilpush.apple.com.example", false},
		{"evilpush.apple.com", false},
		{"push.apple.com.example", false},
		{"fcm.googleapis.com", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := IsAppleHost(tc.host); got != tc.want {
				t.Fatalf("IsAppleHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}
