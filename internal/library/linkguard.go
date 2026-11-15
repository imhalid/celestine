//go:build !js

package library

import (
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// linkTransport is the transport of linkClient: it connects straight to a link's site, with no proxy in between, and
// only to a public address (linkDialControl), so a name that resolves to this machine or its network, at first, after
// a redirect or after a change of its DNS, is not connected to (#532).
func linkTransport() http.RoundTripper {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil // the dialer would see the proxy's address, not the site's
	tr.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   linkDialControl,
	}).DialContext
	return tr
}

// linkDialControl refuses a connection to an address that is not public (publicAddr). It is called with the address
// a name resolved to, just before the connection is made.
func linkDialControl(network, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return err
	}
	if !publicAddr(ap.Addr().WithZone("")) {
		return errPrivateAddr
	}
	return nil
}
