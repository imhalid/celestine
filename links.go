package main

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
)

// Links: a run of text with an address. A click on it goes there: an address
// of the app, page://collection/page (the folders of the page, as agents
// write them), opens that page; a web address, an e-mail, the system's own
// program for it.

// linkHit is a link drawn this frame, where it is in the window.
type linkHit struct {
	r   ui.Rect
	url string
}

// linkSpan is a run of a paragraph that links: the runes it takes.
type linkSpan struct {
	from, to int
	url      string
}

// linkSpans are the links of a paragraph.
func linkSpans(p richtext.Paragraph) []linkSpan {
	var out []linkSpan
	pos := 0
	for _, r := range p.Runs {
		n := utf8.RuneCountInString(r.Text)
		if r.Attributes.Link != "" && n > 0 && r.Attributes.Attachment == nil {
			out = append(out, linkSpan{pos, pos + n, r.Attributes.Link})
		}
		pos += n
	}
	return out
}

// linkAt is the link at (x, y) of the window, as the last frame drew it.
func (a *app) linkAt(x, y float32) (string, bool) {
	for _, h := range a.linkSeen {
		if x >= h.r.X && x < h.r.X+h.r.W && y >= h.r.Y && y < h.r.Y+h.r.H {
			return h.url, true
		}
	}
	return "", false
}

// openLink goes where the address says.
func (a *app) openLink(url string) {
	switch {
	case strings.HasPrefix(url, "page://"):
		path := strings.Trim(strings.TrimPrefix(url, "page://"), "/")
		if !a.openByPath(path) {
			a.notice = "Bağlantının gittiği sayfa yok: " + path
		}
	case strings.HasPrefix(url, "http://"), strings.HasPrefix(url, "https://"), strings.HasPrefix(url, "mailto:"):
		go mygo.Shell.OpenExternal(url)
	}
}

// openByPath opens the page or the collection that the folders of path name:
// "koleksiyon/sayfa/alt-sayfa", from the library's root; failing that, the
// one folder of that last name there is. It reports whether there was one.
func (a *app) openByPath(path string) bool {
	root := strings.TrimSuffix(filepath.ToSlash(a.lib.Root), "/") + "/"
	last := path[strings.LastIndex(path, "/")+1:]
	var byName *Page
	names := 0
	for _, c := range a.lib.Collections {
		if rel := strings.TrimPrefix(filepath.ToSlash(c.Dir), root); rel == path {
			a.openCollection(c)
			return true
		}
		for _, p := range c.All() {
			if strings.TrimPrefix(filepath.ToSlash(p.Dir), root) == path {
				a.open(p)
				return true
			}
			if filepath.Base(p.Dir) == last {
				byName, names = p, names+1
			}
		}
	}
	if names == 1 {
		a.open(byName)
		return true
	}
	return false
}
