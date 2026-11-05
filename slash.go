package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The / and @ menu, under the caret, as Thymer's: one menu, whichever sign
// opens it, at the start of a word. Suggestions (a task, headings, a list),
// the kinds of line (the line becomes the one chosen), dates (today,
// tomorrow… or a day typed as 2026-11-05), the page's choices (Durum: Bitti),
// and the pages to link (a chip; a new page when none is called so).
// What is typed after the sign narrows all of them together - the pages
// first after an @, the kinds first after a /; the list is flat, grouped,
// and scrolls when the window has no room for it (Thymer's submenus, →,
// are not done).  ↑↓ move, ↵ chooses, Esc
// closes. The editor keeps those keys: the menu reads what they did - a
// new line after the query is ↵, the caret leaving the line is ↑ or ↓ - and
// takes it back.

type slashMenu struct {
	open   bool
	sign   string // "/" or "@"
	start  int    // where the sign is
	sel    int
	seen   int // the choice the list was last scrolled to show
	at     int // the caret, last frame
	scroll ui.ScrollState
}

// menuItem is a line of the menu.
type menuItem struct {
	title, hint, group string
	sub                string // faint, under the title
	glyph              *ui.SVG
	run                func()
}

// slashItems are the kinds of block for the query, best first.
func slashItems(q string) []struct{ kind, name, md string } {
	var out []struct{ kind, name, md string }
	for _, k := range blockKinds {
		if q == "" || match(q, k.name) >= 0 || strings.HasPrefix(k.md, q) {
			out = append(out, k)
		}
	}
	return out
}

type namedDay struct {
	name string
	d    time.Time
}

// suggested are the kinds shown first, before anything is typed.
var suggested = map[string]bool{"tasks": true, "h1": true, "h2": true, "bullets": true}

// menuItems are the lines of the menu open, for what is typed after its sign.
func (a *app) menuItems(q string) []menuItem {
	start := a.slash.start
	tq := strings.TrimSpace(q)
	var kinds, picks, days, pages, props, fresh []menuItem
	// the kinds of line
	for _, k := range slashItems(q) {
		k := k
		it := menuItem{title: k.name, group: "Satırı biçimlendir", run: func() { a.chooseSlash(k) }}
		if q == "" && suggested[k.kind] {
			it.group = "Öneriler"
			picks = append(picks, it)
		} else {
			kinds = append(kinds, it)
		}
	}
	// dates
	today := a.clock()
	list := []namedDay{{"Bugün", today}, {"Yarın", today.AddDate(0, 0, 1)}, {"Dün", today.AddDate(0, 0, -1)}, {"Gelecek hafta", today.AddDate(0, 0, 7)}}
	if d, err := time.Parse("2006-01-02", tq); err == nil {
		list = append([]namedDay{{shortDate(d), d}}, list...)
	}
	for _, d := range list {
		if q != "" && match(q, d.name) < 0 && !strings.HasPrefix(d.d.Format("2006-01-02"), tq) {
			continue
		}
		ch := &chip{kind: "date", target: d.d.Format("2006-01-02")}
		days = append(days, menuItem{title: d.name, hint: shortDate(d.d), group: "Tarih", glyph: iconCalendar, run: func() { a.insertChip(start, ch) }})
	}
	// pages
	exact, n := false, 0
	for _, it := range a.places() {
		if it.to.kind != "page" || (a.page != nil && it.to.dir == a.page.Dir) || (q != "" && match(q, it.title) < 0) {
			continue
		}
		if strings.EqualFold(it.title, tq) {
			exact = true
		}
		if n++; n > 6 {
			continue
		}
		title := it.title
		pages = append(pages, menuItem{title: title, sub: it.sub, group: "Bağlantı", glyph: iconPage, run: func() { a.insertChip(start, &chip{kind: "page", target: title}) }})
	}
	// the page's choices
	if p := a.page; p != nil {
		for i := range p.Coll.Properties {
			pr := &p.Coll.Properties[i]
			if pr.Type != PropChoice {
				continue
			}
			for _, o := range pr.Options {
				label := pr.Name + ": " + o
				if q != "" && match(q, label) < 0 {
					continue
				}
				id, o := pr.ID, o
				props = append(props, menuItem{title: label, group: "Özellik", glyph: iconSettings, run: func() {
					a.dropQuery(start)
					a.setValue(p, id, o)
				}})
			}
		}
	}
	// last, a page of what is typed, when none is called so
	if tq != "" && !exact && a.page != nil {
		fresh = append(fresh, menuItem{title: "Yeni sayfa: " + tq, group: "Yeni", glyph: iconPlus, run: func() {
			if _, err := a.page.Coll.NewPage(nil, tq); err != nil {
				a.saveErr = err.Error()
				return
			}
			a.insertChip(start, &chip{kind: "page", target: tq})
		}})
	}
	var out []menuItem
	add := func(g ...[]menuItem) {
		for _, l := range g {
			out = append(out, l...)
		}
	}
	switch {
	case q == "":
		add(picks, kinds, days, props, pages)
	case a.slash.sign == "@":
		add(pages, days, props, kinds, fresh)
	default:
		add(kinds, days, props, pages, fresh)
	}
	if q == "" {
		add(fresh)
	}
	return out
}

// dropQuery takes away the sign and what was typed after it.
func (a *app) dropQuery(start int) {
	_, caret := a.doc.Selection()
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, caret, richtext.Text{})
		ed.Select(start, start)
	})
}

// slashQuery is what is typed after the sign.
func (a *app) slashQuery() string {
	_, caret := a.doc.Selection()
	return a.slashQueryAt(a.doc.Text(), caret)
}

func (a *app) slashQueryAt(text richtext.Text, caret int) string {
	s := &a.slash
	if caret <= s.start+1 {
		return ""
	}
	_, pe := text.ParagraphRange(s.start)
	return text.Slice(s.start+1, min(caret, pe)).String()
}

// trackSlash opens, follows and closes the menu, after an edit or a move
// of the caret; it reports whether it changed the text.
func (a *app) trackSlash(text richtext.Text, caret int, edited, typed bool) bool {
	s := &a.slash
	if !s.open {
		if !typed || caret == 0 {
			return false
		}
		sign := text.Slice(caret-1, caret).String()
		ps, _ := text.ParagraphRange(caret - 1)
		if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-1).Code {
			return false
		}
		if sign != "/" && sign != "@" {
			return false
		}
		if ps != caret-1 && !strings.ContainsAny(text.Slice(caret-2, caret-1).String(), " \t(") {
			return false
		}
		*s = slashMenu{open: true, sign: sign, start: caret - 1, at: caret}
		return false
	}
	if s.start >= text.Len() || text.Slice(s.start, s.start+1).String() != s.sign {
		s.open = false
		return false
	}
	ps, pe := text.ParagraphRange(s.start)
	q := a.slashQueryAt(text, caret)
	items := a.menuItems(q)
	switch {
	case edited && caret > pe && pe < text.Len() && caret == pe+1:
		// ↵: the line was broken after the query; it is joined back, and
		// the choice made
		a.doc.Edit(func(ed *richtext.Edit) { ed.Replace(pe, pe+1, richtext.Text{}) })
		a.doc.SetSelection(pe, pe)
		if len(items) > 0 {
			items[min(s.sel, len(items)-1)].run()
		}
		s.open = false
		return true
	case !edited && (caret < ps || caret > pe):
		// ↑ or ↓: the caret goes back, the choice moves
		if caret < ps {
			s.sel = max(s.sel-1, 0)
		} else {
			s.sel = min(s.sel+1, max(len(items)-1, 0))
		}
		a.doc.SetSelection(s.at, s.at)
		return false
	case caret <= s.start || caret > pe:
		s.open = false
	case strings.HasPrefix(q, " ") || len([]rune(q)) > 40 || strings.HasSuffix(q, "  "):
		s.open = false
	case len(items) == 0 && len([]rune(q)) > 3:
		s.open = false
	}
	s.at = caret
	return false
}

// chooseSlash takes the "/…" away and turns the line into the kind chosen.
func (a *app) chooseSlash(k struct{ kind, name, md string }) {
	s := &a.slash
	_, caret := a.doc.Selection()
	start := s.start
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.Select(start, start)
	})
	if k.kind == "hr" {
		a.doc.Edit(func(ed *richtext.Edit) {
			ed.Replace(start, start, richtext.NewText("\n"))
			ed.FormatParagraphs(start, start, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{ID: hrID} })
			ed.FormatParagraphs(start+1, start+1, func(p *richtext.ParagraphStyle) { *p = richtext.ParagraphStyle{} })
			ed.Select(start+1, start+1)
		})
	} else {
		turnInto(a.doc, start, k.kind)
	}
	a.doc.SetTypingAttributes(richtext.Attributes{})
	s.open = false
}

// The menu's rows have fixed heights, so its height is known before it is
// laid out, and it can be put above the caret when there is no room below.
const (
	menuRow  = 34
	menuRow2 = 46 // a row with a second line
	menuHead = 26
	menuPad  = 6
	menuW    = 300
	menuGap  = 6
)

// menuRows places each row of the menu in its content: where it starts and
// ends, and the content's height.
func menuRows(items []menuItem) (tops, ends []float32, total float32) {
	group := ""
	for i, it := range items {
		if i > 0 {
			total++
		}
		if it.group != group {
			group = it.group
			total += menuHead + 1
		}
		h := float32(menuRow)
		if it.sub != "" {
			h = menuRow2
		}
		tops, ends = append(tops, total), append(ends, total+h)
		total += h
	}
	if len(items) == 0 {
		total = menuRow
	}
	return
}

// slashView is the menu under the caret.
func (a *app) slashView(c *ui.Context) {
	s := &a.slash
	if !s.open || a.doc == nil || a.coll != nil || a.trashOpen {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		s.open = false
		return
	}
	items := a.menuItems(a.slashQuery())
	// ↑ ↓ the editor had nowhere to go with (the first or last line)
	if c.Shortcut(0, ui.KeyDown) {
		s.sel++
	}
	if c.Shortcut(0, ui.KeyUp) {
		s.sel--
	}
	s.sel = max(0, min(s.sel, len(items)-1))
	t := ryugo.From(c)
	cl := t.Colors
	cr := a.caretWin
	// where: below the caret, above it when there is more room there, at
	// most the window's height, and inside its width
	ww, wh := c.Size()
	tops, ends, total := menuRows(items)
	chrome := float32(2*menuPad + 2) // the padding and the border
	below := wh - (cr.Y + cr.H + menuGap) - 8 - chrome
	above := cr.Y - menuGap - 8 - chrome
	inner, top := min(total, below), cr.Y+cr.H+menuGap
	if total > below && above > below {
		inner = min(total, above)
		top = cr.Y - menuGap - inner - chrome
	}
	inner = max(inner, menuRow)
	left := max(8, min(cr.X-4, ww-menuW-8))
	// the choice stays in sight
	if s.sel != s.seen && len(items) > 0 {
		if tops[s.sel] < s.scroll.Y {
			s.scroll.Y = tops[s.sel]
		} else if ends[s.sel] > s.scroll.Y+inner {
			s.scroll.Y = ends[s.sel] - inner
		}
	}
	s.seen = s.sel
	ui.Overlay(c, func() {
		panel := ui.Column(c.Key("slash")).Absolute().Left(left).Top(top).Width(menuW).Padding(menuPad).
			Radius(t.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label("Öneriler")
		t.Shadows.ShadowPop.Apply(panel)
		panel.Children(func() {
			ui.Scroll(c).TrackScroll(&s.scroll).Height(inner).Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					if len(items) == 0 {
						ui.Text(c, "Eşleşen yok").FontSize(13).TextColor(cl.Muted).Height(menuRow).Padding(0, 10).AlignItems(ui.Center)
					}
					group := ""
					for i, it := range items {
						if it.group != group {
							group = it.group
							ui.Text(c.Key("g:"+group), group).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).
								Height(menuHead).Padding(8, 10, 0, 10)
						}
						h := float32(menuRow)
						if it.sub != "" {
							h = menuRow2
						}
						row := ui.Row(c.Key(i)).Height(h).Padding(0, 10).Gap(10).Radius(t.R(ryugo.RadiusControl) - 4).
							AlignItems(ui.Center).Label(it.title).Role(ui.RoleButton)
						text, muted := cl.Text, cl.Muted
						if i == s.sel {
							row.Background(cl.Accent)
							text, muted = cl.OnAccent, cl.OnAccent.Alpha(0.75)
						} else if row.Hovered() {
							row.Background(cl.Fill)
						}
						row.Children(func() {
							if it.glyph != nil {
								ui.Icon(c, it.glyph).Width(15).Height(15).TextColor(muted)
							}
							ui.Column(c).Grow(1).Shrink(1).Children(func() {
								ui.Text(c, it.title).FontSize(14).TextColor(text).MaxLines(1)
								if it.sub != "" {
									ui.Text(c, it.sub).FontSize(11.5).TextColor(muted).MaxLines(1)
								}
							})
							if it.hint != "" {
								ui.Text(c, it.hint).FontSize(12.5).TextColor(muted).MaxLines(1)
							}
						})
						if row.Clicked() {
							a.doc.SetSelection(s.at, s.at)
							it.run()
							s.open = false
						}
					}
				})
			})
		})
	})
}
