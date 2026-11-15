package library

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Links: what a web page says of itself, for a link to it to show - its title, its Open Graph picture and its site's
// icon (#495). Only the page's head is read, and only so much of it.

// LinkMeta is what a web page says of itself: its title, its Open Graph picture and its site's icon, the two last
// addresses made absolute; each "" when the page has none.
type LinkMeta struct {
	Title, Image, Icon string
}

const (
	linkPageMax  = 512 << 10 // what is read of a page for its head
	linkImageMax = 4 << 20   // the largest picture taken for a link
)

// linkClient fetches pages and pictures for links: briefly, as a link waits for them. Its transport (linkTransport)
// connects only to public addresses, and each redirect is checked as the first address was (#532).
var linkClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: linkTransport(),
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		_, err := webURL(req.URL.String())
		return err
	},
}

// AllowLocalLinks lets links be fetched from this machine's loopback addresses: only a test does, for a server of
// its own. Private, link-local and the other addresses of a network are refused all the same.
var AllowLocalLinks = false

// errNotWeb is an address that is not a web page's.
var errNotWeb = errors.New("not a web address")

// errPrivateAddr is an address of this machine or of its network, which a link is not fetched from (#532).
var errPrivateAddr = errors.New("a private address")

// notPublic are the blocks of addresses that are not the public internet's beyond those netip names: "this network"
// (0.0.0.0/8), carrier-grade NAT's shared space (RFC 6598), the IETF's protocol assignments (192.0.0.0/24), the
// benchmarking block (198.18.0.0/15), the reserved 240.0.0.0/4 with the broadcast address, the IPv4-compatible
// addresses (::/96, deprecated), NAT64's local-use block (64:ff9b:1::/48) and the deprecated site-local fec0::/10.
var notPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("fec0::/10"),
}

// nat64 and sixToFour are IPv6 blocks that carry an IPv4 address inside them: NAT64's well-known prefix (RFC 6052)
// in the last four bytes, 6to4 (RFC 3056) in the four after its prefix.
var (
	nat64     = netip.MustParsePrefix("64:ff9b::/96")
	sixToFour = netip.MustParsePrefix("2002::/16")
)

// publicAddr is whether ip is an address of the public internet, which a link may be fetched from: not this machine's
// (loopback, unspecified), not a private network's (RFC 1918, unique-local fc00::/7, carrier-grade NAT), not
// link-local, multicast, reserved or broadcast (notPublic). An IPv4 address mapped into IPv6, or carried in a NAT64 or
// 6to4 address, is judged as itself. Loopback is let through when AllowLocalLinks is set.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch {
	case !ip.IsValid(), ip.IsUnspecified(), ip.IsPrivate(), ip.IsLinkLocalUnicast(), ip.IsMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsLinkLocalMulticast():
		return false
	case ip.IsLoopback():
		return AllowLocalLinks
	case nat64.Contains(ip):
		b := ip.As16()
		return publicAddr(netip.AddrFrom4([4]byte(b[12:16])))
	case sixToFour.Contains(ip):
		b := ip.As16()
		return publicAddr(netip.AddrFrom4([4]byte(b[2:6])))
	}
	for _, p := range notPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// webURL is raw as an address of the web: http or https, with a host that is not this machine's nor a private
// network's when it is written as an address (or as localhost). A name is checked again when it is connected to,
// against the addresses it resolves to (linkTransport).
func webURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errNotWeb
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return nil, errNotWeb
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		if !AllowLocalLinks {
			return nil, errPrivateAddr
		}
	} else if ip, err := netip.ParseAddr(host); err == nil && !publicAddr(ip.WithZone("")) {
		return nil, errPrivateAddr
	}
	return u, nil
}

// linkGet asks for the address with ctx, as a browser would, and returns the answer when it is a success.
func linkGet(ctx context.Context, u *url.URL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Celestine)")
	req.Header.Set("Accept", accept)
	resp, err := linkClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		return nil, errors.New(resp.Status)
	}
	return resp, nil
}

// FetchLinkMeta reads the head of the web page at raw (http or https) for its title, its Open Graph picture and its
// icon. A page that names no icon has its site's /favicon.ico; one that is not HTML has only that.
func FetchLinkMeta(ctx context.Context, raw string) (LinkMeta, error) {
	u, err := webURL(raw)
	if err != nil {
		return LinkMeta{}, err
	}
	resp, err := linkGet(ctx, u, "text/html,application/xhtml+xml")
	if err != nil {
		return LinkMeta{}, err
	}
	defer resp.Body.Close()
	base := resp.Request.URL // where the redirects ended
	var m LinkMeta
	if kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); kind == "" || strings.Contains(kind, "html") {
		m = parseLinkMeta(io.LimitReader(resp.Body, linkPageMax), base)
	}
	if m.Icon == "" {
		m.Icon = base.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	}
	return m, nil
}

// parseLinkMeta reads the head of an HTML page at base: its title (og:title before <title>), its picture
// (og:image before twitter:image) and its icon (a plain icon before Apple's, which is larger, and either before an
// SVG one, which is not drawn), with the addresses made absolute.
func parseLinkMeta(r io.Reader, base *url.URL) LinkMeta {
	z := html.NewTokenizer(r)
	var title, ogTitle, ogImage, twImage, icon string
	iconRank, inTitle := 0, false
	abs := func(ref string) string {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return ""
		}
		u, err := base.Parse(ref)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return ""
		}
		return u.String()
	}
read:
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			break read
		case html.TextToken:
			if inTitle {
				title += string(z.Text())
			}
		case html.EndTagToken:
			switch name, _ := z.TagName(); string(name) {
			case "title":
				inTitle = false
			case "head":
				break read
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, more := z.TagName()
			attrs := map[string]string{}
			for more {
				var k, v []byte
				k, v, more = z.TagAttr()
				attrs[string(k)] = string(v)
			}
			switch string(name) {
			case "title":
				inTitle = tt == html.StartTagToken
			case "body":
				break read
			case "meta":
				key := attrs["property"]
				if key == "" {
					key = attrs["name"]
				}
				content := strings.TrimSpace(attrs["content"])
				switch strings.ToLower(key) {
				case "og:title":
					ogTitle = content
				case "og:image", "og:image:url", "og:image:secure_url":
					if ogImage == "" {
						ogImage = content
					}
				case "twitter:image", "twitter:image:src":
					if twImage == "" {
						twImage = content
					}
				}
			case "link":
				rank := 0
				rels := strings.Fields(strings.ToLower(attrs["rel"]))
				for _, rel := range rels {
					switch rel {
					case "icon":
						rank = max(rank, 3)
					case "apple-touch-icon", "apple-touch-icon-precomposed":
						rank = max(rank, 2)
					}
				}
				href := attrs["href"]
				if rank > 0 && (strings.Contains(attrs["type"], "svg") || strings.HasSuffix(strings.ToLower(strings.SplitN(href, "?", 2)[0]), ".svg")) {
					rank = 1
				}
				if rank > iconRank && abs(href) != "" {
					icon, iconRank = abs(href), rank
				}
			}
		}
	}
	m := LinkMeta{Title: strings.Join(strings.Fields(title), " "), Image: abs(ogImage), Icon: icon}
	if ogTitle != "" {
		m.Title = strings.Join(strings.Fields(ogTitle), " ")
	}
	if m.Image == "" {
		m.Image = abs(twImage)
	}
	return m
}

// FetchLinkImage reads the picture at raw (http or https) for a link: an icon, or a page's Open Graph picture. One
// larger than linkImageMax is not taken.
func FetchLinkImage(ctx context.Context, raw string) ([]byte, error) {
	u, err := webURL(raw)
	if err != nil {
		return nil, err
	}
	resp, err := linkGet(ctx, u, "image/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, linkImageMax+1))
	if err != nil {
		return nil, err
	}
	if len(data) > linkImageMax {
		return nil, errors.New("the picture is too large")
	}
	return data, nil
}
