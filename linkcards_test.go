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
	tt.Advance(300 * time.Millisecond) // the card stays a moment, for the pointer to get onto it
	tt.Frame()
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

// TestLinkCardEditRemove: a link's card has its address at its foot with Edit, which opens the bar's field with the
// address in it to change, and Remove, which takes the link away and leaves the text; the field's link set, its words
// have their flourish (#509).
func TestLinkCardEditRemove(t *testing.T) {
	const addr = "https://example.com/x"
	a, tt := newTestApp(t, "Read [the book]("+addr+") now\n")
	hover := func() {
		t.Helper()
		tt.Frame()
		tt.Advance(100 * time.Millisecond)
		if len(a.linkSeen) == 0 {
			t.Fatal("the link was not drawn")
		}
		r := a.linkSeen[0].r
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(100 * time.Millisecond)
		waitFor(t, tt, "no card", func() bool { return tt.HasText("example.com/x") })
	}
	hover()
	if err := tt.Click("Bağlantının adresini değiştir"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if !a.sel.link || a.sel.url != addr || a.sel.linkSel != [2]int{5, 13} {
		t.Fatalf("the field did not open on the link: %v %q %v", a.sel.link, a.sel.url, a.sel.linkSel)
	}
	if len(a.rangeRects(a.editorR, a.sel.linkSel[0], a.sel.linkSel[1])) == 0 {
		t.Fatal("the text the link is for is not shown")
	}
	typeEach(tt, "/y")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(100 * time.Millisecond)
	if got := mdOf(a.doc); got != "Read [the book]("+addr+"/y) now" {
		t.Fatalf("not changed: %q", got)
	}
	if a.sel.wonAt.IsZero() {
		t.Fatal("no flourish for the link set")
	}
	for range 30 {
		tt.Advance(50 * time.Millisecond)
	}
	if !a.sel.wonAt.IsZero() {
		t.Fatal("the flourish did not end")
	}
	tt.Move(5, 5)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	hover()
	if err := tt.Click("Bağlantıyı kaldır, metin kalsın"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(100 * time.Millisecond)
	if got := mdOf(a.doc); got != "Read the book now" {
		t.Fatalf("not removed: %q", got)
	}
}

// TestLinkIconsSetting: with the site icons off, a web link leaves no room for its icon, and its card still shows
// (#509).
func TestLinkIconsSetting(t *testing.T) {
	a, tt := newTestApp(t, "Read [the book](https://example.com/x) now\n")
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	padded := a.linkSeen[0].r
	a.settings.NoLinkIcons = true
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	plain := a.linkSeen[0].r
	if plain.X >= padded.X-a.font.Size {
		t.Fatalf("the room stayed with the icons off: %v, was %v", plain.X, padded.X)
	}
	tt.Move(plain.X+plain.W/2, plain.Y+plain.H/2)
	tt.Advance(100 * time.Millisecond)
	waitFor(t, tt, "no card", func() bool { return tt.HasText("example.com/x") })
}

// TestPasteLinkOverSelection: an address alone pasted over text selected makes it a link, the text kept, with its
// flourish; other text pasted over it replaces it as before (#509).
func TestPasteLinkOverSelection(t *testing.T) {
	a, tt := newTestApp(t, "Bir iki üç\n")
	tt.Frame()
	clip := " https://example.com/x \n"
	a.clipRead = func() string { return clip }
	a.doc.SetSelection(4, 7)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Advance(50 * time.Millisecond)
	if got := mdOf(a.doc); got != "Bir [iki](https://example.com/x) üç" {
		t.Fatalf("not linked: %q", got)
	}
	if a.sel.wonAt.IsZero() {
		t.Fatal("no flourish for the link pasted")
	}
	// a Mac's menu bar sends ⌘V to the editor as the command "paste", not as the key
	clip = "https://www.memurjet.com/indir"
	a.doc.SetSelection(0, 3)
	tt.Frame()
	tt.Command("paste")
	tt.Advance(50 * time.Millisecond)
	if got := mdOf(a.doc); got != "[Bir](https://www.memurjet.com/indir) [iki](https://example.com/x) üç" {
		t.Fatalf("the menu's paste did not link: %q", got)
	}
	for _, c := range []string{"two words", "example.com"} {
		if pastedAddress(c) {
			t.Fatalf("%q taken for an address", c)
		}
	}
}

// TestLinkIconOnce: a web link that wraps has its icon's room before its first line alone, not before each (#509).
func TestLinkIconOnce(t *testing.T) {
	a, tt := newTestApp(t, "[Kitabın en güçlü fikri şu: insanlar ortak hikâyelere inandıkları için büyük gruplar halinde iş birliği yapabilir. Bunu okurken kendi işimi düşündüm, çok iş görüyor.](https://example.com/x)\n")
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if len(a.linkSeen) < 2 {
		t.Fatalf("the link does not wrap: %d lines", len(a.linkSeen))
	}
	first, second := a.linkSeen[0].r, a.linkSeen[1].r
	if second.X >= first.X-a.font.Size {
		t.Fatalf("the second line has the room too: %v, the first %v", second.X, first.X)
	}
}

// TestPasteLinkFromMenu: the editor's right-click menu's Paste links the text selected with an address, as ⌘V does
// (#509).
func TestPasteLinkFromMenu(t *testing.T) {
	a, tt := newTestApp(t, "Bir iki üç\n")
	tt.SetSize(1100, 700)
	tt.Advance(300 * time.Millisecond)
	a.clipRead = func() string { return "https://www.memurjet.com/indir" }
	a.doc.SetSelection(4, 7)
	tt.Frame()
	r := a.editorR
	tt.Move(r.X+40, r.Y+12)
	tt.Frame()
	tt.RightClickAt(r.X+40, r.Y+12)
	tt.Frame()
	if !a.ctx.open {
		t.Fatal("no menu")
	}
	var paste func()
	for _, it := range a.ctx.items {
		if it.title == "common_paste" {
			paste = it.run
		}
	}
	if paste == nil {
		t.Fatal("no Paste in the menu")
	}
	a.ctx.open = false
	paste()
	tt.Frame()
	if got := mdOf(a.doc); got != "Bir [iki](https://www.memurjet.com/indir) üç" {
		t.Fatalf("the menu's Paste did not link: %q", got)
	}
}

// TestLinkIconRTL: a link whose text runs right to left has its icon's room, and its icon, at its right, where it
// starts (Span.PaddingStart, #509).
func TestLinkIconRTL(t *testing.T) {
	for _, c := range []struct {
		text string
		rtl  bool
	}{{"שלום", true}, {"مرحبا", true}, {"Dune", false}, {"12 שלום", true}, {"", false}} {
		if got := startsRTL(c.text); got != c.rtl {
			t.Errorf("startsRTL(%q) = %v", c.text, got)
		}
	}
	rc := ui.Rect{X: 100, Y: 10, W: 50, H: 20}
	if b := linkIconBox(rc, 16, true); b.X < rc.X+rc.W {
		t.Errorf("the icon of a right-to-left link is not at its right: %v", b)
	}
	if b := linkIconBox(rc, 16, false); b.X+b.W > rc.X {
		t.Errorf("the icon of a left-to-right link is not at its left: %v", b)
	}
	a, tt := newTestApp(t, "קרא את [הספר הזה](https://example.com/x) עכשיו\n")
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if len(a.linkSeen) == 0 {
		t.Fatal("the link was not drawn")
	}
	link := a.linkSeen[0].r
	a.settings.NoLinkIcons = true
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	plain := a.linkSeen[0].r
	// the line is right-aligned: room at the link's right moves the link left by it; room at its left would not
	// move it
	linkIcons = true
	pad := linkIconPad("https://example.com/x", a.font.Size)
	if d := plain.X - link.X; d < pad-1 || d > pad+1 {
		t.Errorf("the room is not at the link's right: x %v, without the icon %v, room %v", link.X, plain.X, pad)
	}
}
