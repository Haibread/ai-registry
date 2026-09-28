package tooldiscovery

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"syscall"
)

// ErrBlockedAddress reports that a candidate resolved to an address the
// registry refuses to connect to on a caller's behalf.
var ErrBlockedAddress = errors.New("address not allowed")

// blockedPrefixes are the ranges net/netip's predicates do not already cover.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64 embeds any IPv4, including private ones
}

// guard decides which resolved addresses outbound discovery may dial.
type guard struct {
	allowed []netip.Prefix
}

func (g guard) permits(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range g.allowed {
		if p.Contains(addr) {
			return true
		}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// control runs after DNS resolution, on the exact address about to be dialed,
// so a hostname that resolves (or re-resolves, or redirects) to an internal
// address is refused too.
func (g guard) control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	if !g.permits(addr) {
		return &blockedError{addr: addr.Unmap()}
	}
	return nil
}

type blockedError struct{ addr netip.Addr }

func (e *blockedError) Error() string {
	return fmt.Sprintf("%s is a private or reserved address", e.addr)
}

func (e *blockedError) Is(target error) bool { return target == ErrBlockedAddress }
