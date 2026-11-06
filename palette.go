package main

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The two palettes, a field over the window: ⌘K searches the documents - go to a
// page, a collection or a tag by typing a few letters of it, or a word in a
// page's text; empty, it lists the places last shown. ⌘P is for commands: it
// lists what the app can do, and typed, those that match. Best first, accents
// and case don't matter. ↑↓ move, ↵ goes, Esc closes.

type palette struct {
	open  bool
	cmds  bool // ⌘P: the commands; else ⌘K: the documents
	query string
	sel   int
	hseen int    // the row last hovered
	was   string // the query of the last frame, to start from the top when it changes
}

// palItem is a line of the palette.
type palItem struct {
	title, sub, keys string
	glyph            *ui.SVG
	run              func()
	score            int
	to               place // where it goes, for a page or a collection
}

// openPalette opens the search of the documents (⌘K).
func (a *app) openPalette() {
	a.pal = palette{open: true}
}

// openCommands opens the commands (⌘P).
func (a *app) openCommands() {
	a.pal = palette{open: true, cmds: true}
}

// commands are what the palette does, where the app is now.
func (a *app) commands() []palItem {
	cmd := func(title, keys string, glyph *ui.SVG, run func()) palItem {
		return palItem{title: title, sub: "Komut", keys: keys, glyph: glyph, run: run}
	}
	var out []palItem
	if c := a.currentColl(); c != nil {
		if !c.Dynamic() {
			out = append(out, cmd("Yeni "+pageNoun(c)+": "+c.Name, "⌘N", iconPlus, func() { a.newPage(c, nil) }))
		}
		out = append(out, cmd("Koleksiyon ayarları: "+c.Name, "", iconSettings, func() { a.openCollSettings(c, 0) }))
		if !c.Journal {
			for _, t := range c.allTemplates() {
				t := t
				out = append(out, cmd("Şablondan yeni "+pageNoun(c)+": "+t.Name, "", iconTemplate, func() { a.newPageFrom(c, nil, t.Body) }))
			}
		}
	}
	if p := a.page; p != nil && !a.noPage() {
		if p.Coll.SubPages {
			out = append(out, cmd("Alt sayfa ekle: "+p.Title, "", iconPlus, func() { a.newPage(p.Coll, p) }))
		}
		out = append(out, cmd("Sayfada bul", "⌘F", iconSearch, func() { a.openFind() }))
		out = append(out, cmd("Çöpe taşı: "+p.Title, "", iconTrash, func() { a.trashPage(p) }))
	}
	if c := a.coll; c != nil {
		for i := range c.Views {
			v := c.Views[i]
			out = append(out, cmd("Görünüm: "+v.Label, "", iconStack, func() {
				if err := c.SetView(v.ID); err != nil {
					a.saveErr = err.Error()
				}
			}))
		}
	}
	out = append(out,
		cmd("Görevler", "", iconTask, a.openTasks),
		cmd("Yazma istatistikleri", "", iconPen, func() { a.statsOpen = true }),
		cmd("Markdown klasörü içe aktar…", "", iconExport, a.pickImportFolder),
		cmd("Kitaplıkta ara", "⇧⌘F", iconSearch, func() { a.openSearch(a.searchQ) }),
		cmd("Hızlı not", a.quickKeys(), iconPlus, a.showQuick),
		cmd("Yeni koleksiyon", "⇧⌘N", iconStack, func() {
			if c, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
				a.newPage(c, nil)
			}
		}),
		cmd("Bugünün sayfası", "⌘J", iconCalendar, a.openToday),
		cmd("Rastgele gün", "", iconCalendar, a.openRandomDay),
		cmd("Geri", "⌘[", iconBack, func() { a.step(-1) }),
		cmd("İleri", "⌘]", iconChevron, func() { a.step(1) }),
		cmd("Kenar çubuğunu aç/kapat", "⌘\\", iconSidebar, a.toggleSidebar),
		cmd("Caret'i değiştir (Slime / Mizan)", "⇧⌘M", iconPage, func() { a.setCaret((a.caretKind + 1) % 2) }),
		cmd("Çöp", "", iconTrash, a.openTrash),
		cmd("Ayarlar", "⌘,", iconSettings, func() { a.settingsOpen = true }),
	)
	mcp := "MCP sunucusunu aç"
	if a.settings.MCP {
		mcp = "MCP sunucusunu kapat"
	}
	out = append(out, cmd(mcp, "", iconSettings, func() { a.setMCP(!a.settings.MCP) }))
	for i, t := range appThemes {
		i := i
		out = append(out, cmd("Renk teması: "+t.name, "", iconTheme, func() { a.setAppTheme(i) }))
	}
	return out
}

// currentColl is the collection of what is shown.
func (a *app) currentColl() *Collection {
	switch {
	case a.coll != nil:
		return a.coll
	case a.page != nil:
		return a.page.Coll
	case len(a.lib.Collections) > 0:
		return a.lib.Collections[0]
	}
	return nil
}

// places are the pages and collections to go to.
func (a *app) places() []palItem {
	var out []palItem
	for _, c := range a.lib.Collections {
		c := c
		out = append(out, palItem{title: c.Name, sub: "Koleksiyon", glyph: iconStack, to: place{"coll", c.Dir}, run: func() { a.goTo(place{"coll", c.Dir}) }})
		for _, p := range c.All() {
			p := p
			sub := c.Name
			if crumb := parents(p); crumb != "" {
				sub += " › " + crumb
			}
			out = append(out, palItem{title: p.Title, sub: sub, glyph: iconPage, to: place{"page", p.Dir}, run: func() { a.goTo(place{"page", p.Dir}) }})
		}
	}
	for _, ti := range a.allTags() {
		key := ti.key
		out = append(out, palItem{title: "#" + ti.name, sub: "Etiket · " + strconv.Itoa(len(ti.pages)) + " sayfa", glyph: iconTag, to: place{"tag", key}, run: func() { a.goTo(place{"tag", key}) }})
	}
	return out
}

// recent are the places last shown, the last first, without the one shown now.
func (a *app) recent(n int) []palItem {
	var out []palItem
	seen := map[place]bool{a.here(): true}
	all := a.places()
	for i := len(a.hist) - 1; i >= 0 && len(out) < n; i-- {
		pl := a.hist[i]
		if seen[pl] {
			continue
		}
		seen[pl] = true
		for _, it := range all {
			if it.to == pl {
				out = append(out, it)
				break
			}
		}
	}
	return out
}

// paletteItems are the lines for the query, best first.
func (a *app) paletteItems(q string) []palItem {
	if a.pal.cmds {
		return a.commandItems(q)
	}
	if strings.TrimSpace(q) == "" {
		if r := a.recent(10); len(r) > 0 {
			return r
		}
		all := a.places()
		return all[:min(10, len(all))]
	}
	var out []palItem
	for _, it := range a.places() {
		s := match(q, it.title)
		if s2 := match(q, it.title+" "+it.sub); s < 0 && s2 >= 0 {
			s = s2 / 2 // in where it is, not in its name: less
		}
		if s < 0 {
			continue
		}
		it.score = s
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return utf8.RuneCountInString(out[i].title) < utf8.RuneCountInString(out[j].title)
	})
	if len(out) > 40 {
		out = out[:40]
	}
	// what is in the pages' text, after what is in their names
	if len([]rune(strings.TrimSpace(q))) >= 3 {
		have := map[string]bool{}
		for _, it := range out {
			if it.to.kind == "page" {
				have[it.to.dir] = true
			}
		}
		for _, h := range a.searchPages(q, 8, false) {
			if have[h.page.Dir] {
				continue
			}
			h := h
			var sb strings.Builder
			for _, sg := range h.snip {
				sb.WriteString(sg.text)
			}
			out = append(out, palItem{title: h.page.Title, sub: "Metinde · " + sb.String(), glyph: iconSearch, to: place{"page", h.page.Dir}, run: func() { a.openFound(h) }})
		}
		query := strings.TrimSpace(q)
		out = append(out, palItem{title: "Kitaplıkta ara: " + query, sub: "Komut", keys: "⇧⌘F", glyph: iconSearch, run: func() { a.openSearch(query) }})
	}
	return out
}

// commandItems are the commands for the query, best first; all of them for none.
func (a *app) commandItems(q string) []palItem {
	all := a.commands()
	if strings.TrimSpace(q) == "" {
		return all
	}
	var out []palItem
	for _, it := range all {
		if s := match(q, it.title); s >= 0 {
			it.score = s
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return utf8.RuneCountInString(out[i].title) < utf8.RuneCountInString(out[j].title)
	})
	return out
}

// fold is s for matching: lower case, Turkish letters as their ASCII.
func fold(s string) string { return turkishFold.Replace(strings.ToLower(turkishFold.Replace(s))) }

// match scores how well q matches s: the words of q all in s, nearer the
// start and at the start of words better; the letters of q in order, worse;
// -1 for no match.
func match(q, s string) int {
	q, s = fold(strings.TrimSpace(q)), fold(s)
	if q == "" {
		return 0
	}
	score := 0
	all := true
	for _, w := range strings.Fields(q) {
		i := strings.Index(s, w)
		if i < 0 {
			all = false
			break
		}
		score += 100 - min(i, 60)
		if i == 0 || s[i-1] == ' ' || s[i-1] == '-' || s[i-1] == ':' {
			score += 40
		}
	}
	if all {
		if strings.HasPrefix(s, q) {
			score += 80
		}
		return score + 200
	}
	// the letters in order, as "ypl" in "yapılacaklar"
	j, gaps, last := 0, 0, -1
	qr := []rune(strings.ReplaceAll(q, " ", ""))
	for i, r := range []rune(s) {
		if j < len(qr) && r == qr[j] {
			if last >= 0 {
				gaps += i - last - 1
			}
			last = i
			j++
		}
	}
	if j < len(qr) {
		return -1
	}
	return max(1, 100-gaps*4)
}

// paletteView shows the palette while it is open.
func (a *app) paletteView(c *ui.Context) {
	if !a.pal.open {
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	ui.DialogBase(c.Key("palette"), &a.pal.open, func(backdrop, panel ui.Element) {
		backdrop.Justify(ui.Start).Padding(90, 16, 16, 16).Background(cl.Scrim.Alpha(0.35))
		panel.Width(580).MaxWidthPercent(100).Radius(t.R(ryugo.RadiusLg)).Background(cl.Card).
			Border(1, cl.Hairline).Clip().Label(a.palLabel())
		t.Shadows.ShadowPop.Apply(panel)
		items := a.paletteItems(a.pal.query)
		if a.pal.query != a.pal.was {
			a.pal.sel, a.pal.was = 0, a.pal.query
		}
		a.pal.sel = max(0, min(a.pal.sel, len(items)-1))
		panel.Children(func() {
			ui.Row(c).Padding(14, 16).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, iconSearch).Width(18).Height(18).TextColor(cl.Muted)
				in := ui.TextInputBase(c.Key("palette-in"), &a.pal.query).Label("Ara ya da komut").
					Placeholder("Sayfaya git ya da bir komut yaz…").PlaceholderColor(cl.Muted).
					FontSize(16).Grow(1).FocusRing(false).AutoFocus()
				switch {
				case in.Shortcut(0, ui.KeyDown):
					a.pal.sel = min(a.pal.sel+1, len(items)-1)
				case in.Shortcut(0, ui.KeyUp):
					a.pal.sel = max(a.pal.sel-1, 0)
				case in.Submitted() && len(items) > 0:
					a.runItem(items[a.pal.sel])
				}
			})
			ui.Box(c).Height(1).Background(cl.Hairline)
			ui.Scroll(c).MaxHeight(380).Children(func() {
				ui.Column(c).Padding(6).Gap(1).Children(func() {
					if len(items) == 0 {
						ui.Text(c, "Eşleşen bir şey yok.").FontSize(13.5).TextColor(cl.Muted).Padding(12, 10)
						return
					}
					head := ""
					for i, it := range items {
						if a.pal.query == "" {
							h := "Son açılanlar"
							if a.pal.cmds {
								h = "Komutlar"
							}
							if h != head {
								head = h
								ui.Text(c.Key("head:"+h), h).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Padding(8, 10, 4, 10)
							}
						}
						a.paletteRow(c, i, it)
					}
				})
			})
		})
	})
}

func (a *app) palLabel() string {
	if a.pal.cmds {
		return "Komut ara"
	}
	return "Sayfa ara"
}

func (a *app) palHint() string {
	if a.pal.cmds {
		return "Bir komut yaz…"
	}
	return "Sayfaya git ya da metinde ara…"
}

func (a *app) paletteRow(c *ui.Context, i int, it palItem) {
	t := ryugo.From(c)
	cl := t.Colors
	row := ui.Row(c.Key(i)).Height(40).Padding(0, 10).Gap(10).Radius(t.R(ryugo.RadiusControl) - 4).AlignItems(ui.Center).
		Label(it.title).Role(ui.RoleButton)
	text, muted := cl.Text, cl.Muted
	if row.Hovered() && a.pal.hseen != i { // one row lit: the pointer's, once it comes
		a.pal.hseen, a.pal.sel = i, i
		c.Invalidate()
	}
	if i == a.pal.sel {
		row.LinearGradient(chosenFill(t))
		text, muted = cl.OnAccent, cl.OnAccent.Alpha(0.75)
	}
	row.Children(func() {
		ui.Icon(c, it.glyph).Width(16).Height(16).TextColor(muted)
		ui.Text(c, it.title).FontSize(14).TextColor(text).MaxLines(1).Shrink(1)
		if it.sub != "" && it.sub != "Komut" {
			ui.Text(c, it.sub).FontSize(12.5).TextColor(muted).MaxLines(1).Shrink(1)
		}
		ui.Spacer(c)
		if it.keys != "" {
			ui.Text(c, it.keys).FontSize(12.5).TextColor(muted)
		}
	})
	if row.Clicked() {
		a.runItem(it)
	}
}

func (a *app) runItem(it palItem) {
	a.pal.open = false
	if it.run != nil {
		it.run()
	}
}
