package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// linkSite is a web site of a test's own, with a page that has a title, an Open Graph picture and an icon; links
// are asked for while the test runs (linkFetch).
func linkSite(t *testing.T) *httptest.Server {
	var pic bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	png.Encode(&pic, img)
	mux := http.NewServeMux()
	mux.HandleFunc("/dune", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>Dune</title><meta property="og:title" content="Dune, the novel"><meta property="og:image" content="/og.png"><link rel="icon" href="/fav.png"></head></html>`))
	})
	for _, p := range []string{"/og.png", "/fav.png"} {
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(pic.Bytes())
		})
	}
	srv := httptest.NewServer(mux)
	was := linkFetch
	linkFetch = true
	t.Cleanup(func() { linkFetch = was; srv.Close() })
	return srv
}

// waitFor advances tt until ok, or fails with what.
func waitFor(t *testing.T, tt *ui.Tester, what string, ok func() bool) {
	t.Helper()
	for range 100 {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
		tt.Advance(50 * time.Millisecond)
	}
	t.Fatalf("%s: %v", what, tt.Texts())
}

// TestLinkIcons: a web link leaves room before it for its site's icon, which is read from its page; resting on the
// link shows its card with the page's title and site; with link previews off there is neither (#495).
func TestLinkIcons(t *testing.T) {
	srv := linkSite(t)
	addr := srv.URL + "/dune"
	a, tt := newTestApp(t, "Read [the book]("+addr+") now\n")
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if len(a.linkSeen) == 0 {
		t.Fatal("the link was not drawn")
	}
	padded := a.linkSeen[0].r
	waitFor(t, tt, "no site icon", func() bool { return a.siteIcon(addr) != nil })
	// the card, once the pointer rests on the link
	tt.Move(padded.X+padded.W/2, padded.Y+padded.H/2)
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("Dune, the novel") {
		t.Fatal("the card showed at once")
	}
	waitFor(t, tt, "no card", func() bool { return tt.HasText("Dune, the novel") && tt.HasText("127.0.0.1") })
	tt.Move(5, 5)
	tt.Advance(100 * time.Millisecond)
	if tt.HasText("Dune, the novel") {
		t.Fatal("the card stayed after the pointer left")
	}
	// off: no room before the link, nothing asked
	a.settings.NoLinkPreviews = true
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if plain := a.linkSeen[0].r; plain.X >= padded.X-a.font.Size {
		t.Fatalf("the room stayed with previews off: %v, was %v", plain.X, padded.X)
	}
	if a.linkPage(addr, false) != nil {
		t.Fatal("a page asked for with previews off")
	}
}

// TestLinkFieldPreview: an address typed into the bar's link field shows its page's title and site under it once the
// typing rests, before it is set (#495).
func TestLinkFieldPreview(t *testing.T) {
	srv := linkSite(t)
	a, tt := newTestApp(t, "Bir iki üç\n")
	tt.Frame()
	a.doc.SetSelection(4, 7)
	tt.Frame()
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if err := tt.Click("Bağlantı (⌘K)"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	typeEach(tt, srv.URL+"/dune")
	if tt.HasText("Dune, the novel") {
		t.Fatal("the page was shown while the typing went on")
	}
	waitFor(t, tt, "no preview in the field", func() bool { return tt.HasText("Dune, the novel") })
	tt.Key(0, ui.KeyEnter)
	tt.Advance(200 * time.Millisecond)
	if got := mdOf(a.doc); got != "Bir [iki]("+srv.URL+"/dune) üç" {
		t.Fatalf("not linked: %q", got)
	}
}
