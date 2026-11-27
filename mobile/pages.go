package main

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/internal/textnorm"
	"github.com/imhalid/celestine/mobile/locales"
)

// What the screens show of pages: their words as plain text, read once a version of the file and kept, so that a
// frame reads no file; the pages edited last; a row that shows a page in a list.

// plainCache is a page's words as plain text, as of the page's file's modification.
type plainCache struct {
	mod   time.Time
	plain string
}

// plains are the pages' words, and their Markdown, by page.
var (
	plains = map[*library.Page]plainCache{}
	mds    = map[*library.Page]plainCache{}
)

// mdOf is page p's Markdown, read again only when its file changed.
func mdOf(p *library.Page) string {
	if pc, ok := mds[p]; ok && pc.mod.Equal(p.Modified) {
		return pc.plain
	}
	md, _ := p.Text()
	mds[p] = plainCache{mod: p.Modified, plain: md}
	return md
}

// plainOf is page p's words as plain text, read again only when its file changed.
func plainOf(p *library.Page) string {
	if pc, ok := plains[p]; ok && pc.mod.Equal(p.Modified) {
		return pc.plain
	}
	s := strings.TrimSpace(textnorm.Plain(mdOf(p)))
	plains[p] = plainCache{mod: p.Modified, plain: s}
	return s
}

// preview is the first words of text, on one line, at most n runes.
func preview(text string, n int) string {
	text = strings.Join(strings.Fields(text), " ")
	if rs := []rune(text); len(rs) > n {
		return strings.TrimRightFunc(string(rs[:n]), unicode.IsSpace) + "…"
	}
	return text
}

// lines is the first n lines of text that have words.
func lines(text string, n int) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
		if len(out) == n {
			break
		}
	}
	return strings.Join(out, "\n")
}

// card is a group of rows on the surface, rounded, as a phone's settings are.
func card(c *ui.Context, rows func()) {
	t := c.Theme()
	ui.Column(c).Margin(0, 16).Radius(20).Background(t.Surface).Border(1, t.Border).Children(rows)
}

// wordCount is how many words text has.
func wordCount(text string) int { return len(strings.Fields(text)) }

// allPages is every page of the library's collections, the saved searches' none.
func (a *app) allPages() []*library.Page {
	var out []*library.Page
	for _, c := range a.lib.Collections {
		if !c.Dynamic() {
			out = append(out, c.All()...)
		}
	}
	return out
}

// recentPages are the n pages edited last.
func (a *app) recentPages(n int) []*library.Page {
	ps := a.allPages()
	slices.SortStableFunc(ps, func(x, y *library.Page) int { return y.Modified.Compare(x.Modified) })
	return ps[:min(n, len(ps))]
}

// ago says how long ago t was, as a person would.
func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return locales.EditedNow()
	case d < time.Hour:
		return locales.EditedMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		return locales.EditedHours(int(d / time.Hour))
	case startOfDay(t).Equal(startOfDay(time.Now()).AddDate(0, 0, -1)):
		return locales.EditedYesterday()
	}
	return locales.EditedDays(int(startOfDay(time.Now()).Sub(startOfDay(t)) / (24 * time.Hour)))
}

// titleOf is the title page p shows under: a day of the journal its date written out (its title is the journal's
// format, for files and links).
func titleOf(p *library.Page) string {
	if p.Coll != nil && p.Coll.Journal && p.Parent == nil {
		return formatDay("dddd, D MMMM YYYY", dayOf(p))
	}
	if strings.TrimSpace(p.Title) == "" {
		return locales.PageUntitled()
	}
	return p.Title
}

// pageBlock is a page as a block of a list: its title, its first words, where it is and when it was written. showColl
// says the collection too, for a list of pages of several.
func (a *app) pageBlock(c *ui.Context, p *library.Page, showColl bool) ui.Element {
	b := block(c, p, pal.block, smallR+4).FillWidth().Padding(16, 18).Gap(6).Role(ui.RoleButton).Label(titleOf(p))
	b.Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, titleOf(p)).FontSize(17).FontWeight(700).MaxLines(1).Grow(1).MinWidth(0)
			if p.Locked != "" {
				ui.Icon(c, iconLock).Size(15, 15).TextColor(pal.faint)
			}
		})
		if s := preview(plainOf(p), 160); s != "" {
			ui.Text(c, s).FontSize(15).LineHeight(1.4).TextColor(pal.muted).MaxLines(2).Wrap()
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(4, 0, 0, 0).Children(func() {
			if showColl && p.Coll != nil {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4, 10).Radius(12).Background(pal.accent.Alpha(0.1)).Children(func() {
					ui.Box(c).Size(7, 7).Radius(4).Background(pal.accent)
					ui.Text(c, p.Coll.Name).FontSize(12).FontWeight(650).TextColor(pal.accent)
				})
			}
			ui.Text(c, ago(p.Modified)).FontSize(13).FontWeight(500).TextColor(pal.faint)
		})
	})
	if b.Clicked() {
		a.openPage(p)
	}
	return b
}

// orderedCollections are the collections as the screens list them: the journal first, then as the desktop orders them.
func (a *app) orderedCollections() []*library.Collection {
	out := slices.Clone(a.lib.Collections)
	slices.SortStableFunc(out, func(x, y *library.Collection) int {
		switch {
		case x.Journal == y.Journal:
			return 0
		case x.Journal:
			return -1
		}
		return 1
	})
	return out
}
