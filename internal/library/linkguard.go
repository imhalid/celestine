//go:build !js

package library

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// linkTransport is the transport of linkClient. A request the system sends through a proxy (LinkProxy) goes through
// it: the proxy resolves the site's name, and only the address as written is checked (webURL), at first and at each
// redirect. One that goes straight connects only to a public address (linkDialControl), so a name that resolves to
// this machine or its network, at first, after a redirect or after a change of its DNS, is not connected to (#532).
// A proxy that cannot be found out fails the request: it is not sent straight around a proxy the user set (#559).
func linkTransport() http.RoundTripper {
	direct := http.DefaultTransport.(*http.Transport).Clone()
	direct.Proxy = nil // the dialer would see the proxy's address, not the site's
	direct.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   linkDialControl,
	}).DialContext
	proxied := http.DefaultTransport.(*http.Transport).Clone()
	proxied.Proxy = func(req *http.Request) (*url.URL, error) { return LinkProxy(req) }
	return linkRoundTripper{direct, proxied}
}

// linkRoundTripper sends each request, a redirect's too, straight or through the proxy, as LinkProxy says for it.
type linkRoundTripper struct{ direct, proxied http.RoundTripper }

func (t linkRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if LinkProxy == nil {
		return t.direct.RoundTrip(req)
	}
	p, err := LinkProxy(req)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return t.direct.RoundTrip(req)
	}
	return t.proxied.RoundTrip(req)
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
