package push

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

const (
	// SendTimeout bounds one whole send, dial to last byte.
	SendTimeout = 10 * time.Second
	// DialTimeout bounds the connection attempt alone.
	DialTimeout = 5 * time.Second
)

// ErrRefusedAddress means the dialer refused to connect to a loopback,
// private, link local, multicast or unspecified address.
var ErrRefusedAddress = errors.New("push: refusing to connect to a non public address")

// cgnat is the carrier grade NAT range, which netip does not call private.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// NewGuardedClient is the only HTTP client a push send may use. webpush-go's
// default client has no timeout, follows redirects and dials any address, so a
// hostile or compromised endpoint would become SSRF or a hung worker. This
// client has a 10 second timeout, follows no redirect (a 3xx comes back as the
// response) and refuses internal addresses.
//
// The address check runs in the dialer's Control hook, which sees the address
// after name resolution, so a name that resolves to an internal address (DNS
// rebinding) is refused too.
func NewGuardedClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: DialTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrRefusedAddress
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || refusedAddress(ip) {
				return ErrRefusedAddress
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: SendTimeout,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: DialTimeout,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// refusedAddress reports whether ip must never be dialled.
func refusedAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		(ip.Is4() && cgnat.Contains(ip))
}
