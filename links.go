package main

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Links: a run of text with an address. A click on it goes there: an address
// of the app, page://collection/page (the folders of the page, as agents
// write them), opens that page; a web address, an e-mail, the system's own
// program for it.

// linkHit is a link drawn this frame, where it is in the window.
type linkHit struct {
	r   ui.Rect
	url string
	pos int // where in the text its run starts
}

// linkSpan is a run of a paragraph that links: the runes it takes, and whether it starts its link, which a run
// before it with the same address does not (the room of a web link's icon is before its first, linkRoom).
type linkSpan struct {
	from, to int
	url      string
	first    bool
}

// linkSpans are the links of a paragraph.
func linkSpans(p richtext.Paragraph) []linkSpan {
	var out []linkSpan
	pos, prev := 0, ""
	for _, r := range p.Runs {
		n := utf8.RuneCountInString(r.Text)
		link := r.Attributes.Link
		if r.Attributes.Attachment != nil {
			link = ""
		}
		if link != "" && n > 0 {
			out = append(out, linkSpan{pos, pos + n, link, link != prev})
		}
		if n > 0 {
			prev = link
		}
		pos += n
	}
	return out
}

// linkAt is the link at (x, y) of the window, as the last frame drew it.
func (a *app) linkAt(pn *paneState, x, y float32) (string, bool) {
	for _, h := range pn.linkSeen {
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h.url, true
		}
	}
	return "", false
}

// openExternal opens an address of the web, or a mail, in the system's app: a test sees what it is given instead.
var openExternal = func(url string) { go mygo.Shell.OpenExternal(url) }

// openLinkIn goes where the address says, in pn.
func (a *app) openLinkIn(pn *paneState, url string) {
	switch {
	case strings.HasPrefix(url, "page://"):
		path := strings.Trim(strings.TrimPrefix(url, "page://"), "/")
		if !a.openByPathIn(pn, path) {
			a.say(toastWarn, locales.PageLinkMissingAt(path))
		}
	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "mailto:"):
		openExternal(url)
	}
}

// openLink goes where the address says in the active pane.
func (a *app) openLink(url string) { a.openLinkIn(a.pane(), url) }

// openByPath opens the page or the collection that the folders of path name in the active pane.
func (a *app) openByPath(path string) bool { return a.openByPathIn(a.pane(), path) }

// openByPathIn opens the page or the collection that the folders of path name:
// "koleksiyon/sayfa/alt-sayfa", from the library's root; failing that, the
// one folder of that last name there is. It reports whether there was one.
func (a *app) openByPathIn(pn *paneState, path string) bool {
	root := strings.TrimSuffix(filepath.ToSlash(a.lib.Root), "/") + "/"
	last := path[strings.LastIndex(path, "/")+1:]
	var byName *Page
	names := 0
	for _, c := range a.lib.Collections {
		if rel := strings.TrimPrefix(filepath.ToSlash(c.Dir), root); rel == path {
			a.openCollectionIn(pn, c)
			return true
		}
		for _, p := range c.All() {
			if strings.TrimPrefix(filepath.ToSlash(p.Dir), root) == path {
				a.openIn(pn, p)
				return true
			}
			if filepath.Base(p.Dir) == last {
				byName, names = p, names+1
			}
		}
	}
	if names == 1 {
		a.openIn(pn, byName)
		return true
	}
	return false
}
