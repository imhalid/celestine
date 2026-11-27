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

// pageBlock is page p in a list: its title, its collection as a chip in the collection's color at the top right, and
// the first words of it on a line with when it was written last at the right.
func (a *app) pageBlock(c *ui.Context, p *library.Page, showColl bool) ui.Element {
	b := block(c, p, pal.block, smallR+4).FillWidth().Padding(16, 18).Gap(8).Role(ui.RoleButton).Label(titleOf(p))
	b.Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, titleOf(p)).FontSize(17).FontWeight(700).MaxLines(1).Grow(1).MinWidth(0)
			if p.Locked != "" {
				ui.Icon(c, iconLock).Size(15, 15).TextColor(pal.faint)
			}
			if showColl && p.Coll != nil {
				collChip(c, p.Coll)
			}
		})
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, preview(plainOf(p), 120)).FontSize(15).TextColor(pal.muted).MaxLines(1).Grow(1).MinWidth(0)
			ui.Text(c, when(p.Modified, time.Now())).FontSize(13).FontWeight(550).TextColor(pal.faint).Shrink(0)
		})
	})
	if b.Clicked() {
		a.openPage(p)
	}
	return b
}

// collChip is collection k's name in a chip of its color.
func collChip(c *ui.Context, k *library.Collection) {
	t := tonesOf(k)
	ui.Row(c.Key("chip")).Gap(6).AlignItems(ui.Center).Padding(4, 10).Radius(12).Background(t.ground).Shrink(0).Children(func() {
		ui.Icon(c, collGlyph(k)).Size(13, 13).TextColor(t.color)
		ui.Text(c, k.Name).FontSize(12).FontWeight(650).TextColor(t.ink).MaxLines(1)
	})
}

// when is the day and the time of t, as a list says it: the time alone today.
func when(t, now time.Time) string {
	t = t.Local()
	clock := t.Format("15:04")
	if startOfDay(t).Equal(startOfDay(now)) {
		return clock
	}
	if t.Year() != now.Year() {
		return formatDay("D MMM YYYY", t)
	}
	return formatDay("D MMM", t) + " " + clock
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

// collColors are the colors a collection can have, the desktop's (Ryugo's chart colors, collicons.go of the desktop),
// by their names in the collection's file; "" is none, which takes one of them by the collection's name.
var collColors = []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"}

// chartHues are the hues of c1 to c8, and their chroma on the light look and the dark, as Ryugo's tokens have them.
var chartHues = [8][3]float32{{256, 0.198, 0.159}, {182, 0.103, 0.125}, {68, 0.127, 0.153}, {12, 0.21, 0.195},
	{306, 0.21, 0.2}, {130, 0.156, 0.188}, {29, 0.21, 0.191}, {236, 0.124, 0.15}}

// chartColor is the color named name (c1 to c8) on the look shown, and whether it is one.
func chartColor(name string) (ui.Color, bool) {
	for i, n := range collColors {
		if n == name {
			h := chartHues[i]
			if dark {
				return ui.Oklch(0.7, h[2], h[0]), true
			}
			return ui.Oklch(0.58, h[1], h[0]), true
		}
	}
	return ui.Color{}, false
}

// collColor is collection k's color: its own, or one of the eight by its name (the journal's the seventh, a coral).
func collColor(k *library.Collection) ui.Color {
	if col, ok := chartColor(k.Color); ok {
		return col
	}
	if col, ok := namedColor(k.Color); ok {
		return col
	}
	if k.Journal {
		col, _ := chartColor("c7")
		return col
	}
	h := 0
	for _, r := range k.Name {
		h = h*31 + int(r)
	}
	col, _ := chartColor(collColors[(h%8+8)%8])
	return col
}

// namedColor is a color a collection was given before the chart colors: a name or a hex.
func namedColor(s string) (ui.Color, bool) {
	named := map[string]string{"red": "#E5484D", "orange": "#F2862E", "yellow": "#E0B12C", "green": "#30A46C",
		"teal": "#12A594", "blue": "#3D6EE8", "purple": "#8E4EC6", "pink": "#D6409F", "gray": "#8B8D98", "brown": "#AD7F58"}
	if h, ok := named[s]; ok {
		return ui.Hex(h), true
	}
	if len(s) == 7 && s[0] == '#' {
		return ui.Hex(s), true
	}
	return ui.Color{}, false
}

// tones are a collection's colors, all of its one color: its block's ground, its icon's ground, its icon, its words.
type tones struct{ ground, mark, color, ink ui.Color }

// tonesOf are collection k's tones.
func tonesOf(k *library.Collection) tones {
	col := collColor(k)
	if dark {
		return tones{ground: pal.block.MixOklab(col, 0.2), mark: pal.block.MixOklab(col, 0.38), color: col,
			ink: col.MixOklab(ui.Hex("#FFFFFF"), 0.55)}
	}
	return tones{ground: pal.block.MixOklab(col, 0.13), mark: pal.block.MixOklab(col, 0.26), color: col,
		ink: col.MixOklab(ui.Hex("#000000"), 0.45)}
}

// collGlyph is collection k's icon.
func collGlyph(k *library.Collection) *ui.SVG { return collGlyphOf(k.Icon, k.Journal) }
