package push

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rtFunc turns a function into an http.RoundTripper so tests can stand in for
// the network.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGuardedClientRefusesPrivateAddresses(t *testing.T) {
	t.Run("loopback test server is refused through the client", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("the guarded client reached a loopback server")
		}))
		defer srv.Close()
		resp, err := NewGuardedClient().Get(srv.URL)
		if err == nil {
			resp.Body.Close()
			t.Fatal("expected the dial to be refused")
		}
		if !errors.Is(err, ErrRefusedAddress) {
			t.Fatalf("error = %v, want ErrRefusedAddress in the chain", err)
		}
	})

	transport, ok := NewGuardedClient().Transport.(*http.Transport)
	if !ok || transport.DialContext == nil {
		t.Fatal("guarded client must use an *http.Transport with a custom DialContext")
	}
	cases := []struct {
		name string
		addr string
		v6   bool
	}{
		{name: "loopback v4", addr: "127.0.0.1:443"},
		{name: "loopback v6", addr: "[::1]:443", v6: true},
		{name: "private 10", addr: "10.0.0.1:443"},
		{name: "private 192.168", addr: "192.168.1.1:443"},
		{name: "private 172.16", addr: "172.16.0.1:443"},
		{name: "link local metadata", addr: "169.254.169.254:80"},
		{name: "unspecified v4", addr: "0.0.0.0:443"},
		{name: "carrier grade nat", addr: "100.64.0.1:443"},
		{name: "v4 mapped loopback", addr: "[::ffff:127.0.0.1]:443", v6: true},
		{name: "multicast", addr: "224.0.0.1:443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.v6 {
				ln, err := net.Listen("tcp6", "[::1]:0")
				if err != nil {
					t.Skip("no IPv6 loopback on this machine")
				}
				ln.Close()
			}
			conn, err := transport.DialContext(context.Background(), "tcp", tc.addr)
			if err == nil {
				conn.Close()
				t.Fatalf("dial to %s succeeded, want ErrRefusedAddress", tc.addr)
			}
			if !errors.Is(err, ErrRefusedAddress) {
				t.Fatalf("dial to %s error = %v, want ErrRefusedAddress", tc.addr, err)
			}
		})
	}
}

func TestGuardedClientDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	client := NewGuardedClient()
	client.Transport = rtFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		h := http.Header{}
		h.Set("Location", "https://fcm.googleapis.com/elsewhere")
		return &http.Response{StatusCode: http.StatusFound, Header: h, Body: http.NoBody, Request: r}, nil
	})
	resp, err := client.Get("https://fcm.googleapis.com/start")
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want the 302 returned as is", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("transport called %d times, want 1 (no redirect followed)", calls)
	}
}

func TestGuardedClientTimeout(t *testing.T) {
	client := NewGuardedClient()
	if client.Timeout != SendTimeout {
		t.Fatalf("Timeout = %v, want %v", client.Timeout, SendTimeout)
	}
}
