//go:build !js

package library

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

// TestLinkPrivateAddrs: a link is not fetched from this machine nor from its network - loopback, private, link-local,
// unique-local, unspecified and carrier-grade NAT addresses, in IPv4, IPv6 and IPv4 mapped into IPv6 - whether the
// address is written in the link or is what its name resolves to; a public address is (#532).
func TestLinkPrivateAddrs(t *testing.T) {
	private := []string{
		"127.0.0.1", "127.8.9.1", "::1", // loopback
		"10.0.0.1", "172.16.5.4", "172.31.255.255", "192.168.1.1", // RFC 1918
		"169.254.169.254", "fe80::1", // link-local
		"fc00::1", "fd12:3456::1", // unique-local
		"0.0.0.0", "::", "0.1.2.3", "255.255.255.255", // unspecified, this network, broadcast
		"100.64.0.1", "100.127.255.254", // carrier-grade NAT
		"::ffff:127.0.0.1", "::ffff:10.0.0.1", "::ffff:169.254.169.254", // mapped into IPv6
		"224.0.0.1", "ff02::1", // multicast
		"64:ff9b::a00:1", "64:ff9b::7f00:1", "64:ff9b::a9fe:a9fe", "64:ff9b:1::808:808", // NAT64, and its local-use block
		"2002:c0a8:101::1", "2002:7f00:1::", "2002:a9fe:a9fe::1", // 6to4 of a private address
		"::10.0.0.1", "::8.8.8.8", // IPv4-compatible
		"fec0::1", "feff::1", // site-local
		"240.0.0.1", "250.1.2.3", // reserved
		"198.18.0.1", "198.19.255.255", // benchmarking
		"192.0.0.1", "192.0.0.170", // IETF protocol assignments
	}
	public := []string{
		"93.184.215.14", "1.1.1.1", "100.63.255.255", "100.128.0.1", "172.32.0.1", "2606:4700::1111", "::ffff:8.8.8.8",
		"64:ff9b::808:808", "2002:808:808::1", "198.17.255.255", "198.20.0.1", "192.0.1.1", "223.255.255.1",
	}
	for _, s := range private {
		if publicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s is taken as public", s)
		}
		ap := netip.AddrPortFrom(netip.MustParseAddr(s), 443)
		if err := linkDialControl("tcp", ap.String(), nil); !errors.Is(err, errPrivateAddr) {
			t.Errorf("%s is connected to: %v", ap, err)
		}
		host := s
		if strings.Contains(s, ":") {
			host = "[" + s + "]"
		}
		if _, err := webURL("http://" + host + "/admin"); !errors.Is(err, errPrivateAddr) {
			t.Errorf("a link to %s is fetched: %v", host, err)
		}
	}
	for _, s := range public {
		if !publicAddr(netip.MustParseAddr(s)) {
			t.Errorf("%s is taken as private", s)
		}
		if err := linkDialControl("tcp", netip.AddrPortFrom(netip.MustParseAddr(s), 443).String(), nil); err != nil {
			t.Errorf("%s is refused: %v", s, err)
		}
	}
	for _, raw := range []string{"http://localhost:8080/", "https://LOCALHOST./x", "http://app.localhost/", "http://[fe80::1%25en0]/"} {
		if _, err := webURL(raw); !errors.Is(err, errPrivateAddr) {
			t.Errorf("%s is fetched: %v", raw, err)
		}
	}
	if _, err := webURL("https://example.com/post"); err != nil {
		t.Errorf("a link to a site is refused: %v", err)
	}

	// A name that resolves to this machine is refused when it is connected to, not only when it is written.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<title>in</title>")) }))
	defer srv.Close()
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	if _, err := FetchLinkMeta(context.Background(), srv.URL+"/"); !errors.Is(err, errPrivateAddr) {
		t.Errorf("the test's server is fetched: %v", err)
	}
	if _, err := linkClient.Get("http://localhost:" + port + "/"); !errors.Is(err, errPrivateAddr) {
		t.Errorf("a name for this machine is connected to: %v", err)
	}
}

// TestLinkRedirectPrivate: a page that sends its link on to an address of this machine's network is not followed
// there, for its meta nor for a picture (#532).
func TestLinkRedirectPrivate(t *testing.T) {
	localLinks(t) // the test's own server is on 127.0.0.1; the addresses it sends on to are refused all the same
	mux := http.NewServeMux()
	mux.HandleFunc("/to/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+strings.TrimPrefix(r.URL.Path, "/to/")+"/secret", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := context.Background()
	for _, host := range []string{"10.0.0.1", "192.168.1.1", "169.254.169.254", "[fd00::1]", "[::ffff:192.168.0.1]", "100.64.0.1"} {
		if _, err := FetchLinkMeta(ctx, srv.URL+"/to/"+host); !errors.Is(err, errPrivateAddr) {
			t.Errorf("a redirect to %s is followed: %v", host, err)
		}
		if _, err := FetchLinkImage(ctx, srv.URL+"/to/"+host); !errors.Is(err, errPrivateAddr) {
			t.Errorf("a picture's redirect to %s is followed: %v", host, err)
		}
	}
}

// TestLinkThroughProxy: a link's page is asked through the proxy the system says, which resolves the site's name (the
// proxy's own address, here this machine's, is not judged); a proxy that cannot be found out fails the request rather
// than sending it straight; an address written as a private one is refused before any proxy is asked (#559).
func TestLinkThroughProxy(t *testing.T) {
	var asked []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.String()) // a proxy is asked for the whole address
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><head><title>Through the proxy</title></head></html>"))
	}))
	defer proxy.Close()
	via, _ := url.Parse(proxy.URL)
	LinkProxy = func(*http.Request) (*url.URL, error) { return via, nil }
	defer func() { LinkProxy = nil }()
	m, err := FetchLinkMeta(context.Background(), "http://example.com/page")
	if err != nil || m.Title != "Through the proxy" || len(asked) != 1 || asked[0] != "http://example.com/page" {
		t.Fatalf("through the proxy: %v %+v, the proxy asked for %v", err, m, asked)
	}
	LinkProxy = func(*http.Request) (*url.URL, error) { return nil, errors.New("no answer from the system") }
	if _, err := FetchLinkMeta(context.Background(), "http://example.com/page"); err == nil || len(asked) != 1 {
		t.Fatalf("a proxy not found out: %v, the proxy asked %d times", err, len(asked))
	}
	LinkProxy = func(*http.Request) (*url.URL, error) { return via, nil }
	if _, err := FetchLinkMeta(context.Background(), "http://192.168.1.1/"); !errors.Is(err, errPrivateAddr) || len(asked) != 1 {
		t.Fatalf("a private address through the proxy: %v", err)
	}
}
