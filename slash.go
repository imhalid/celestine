package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The / and @ menus, under the caret, as Thymer's.
//   - "/" at the start of a line: the kinds of block; the line becomes the
//     one chosen.
//   - "@" at the start of a word: a link to a page (a chip; a new page when
//     none is called so), a date (today, tomorrow… or a day typed as
//     2026-11-05), or a value of one of the page's choices (Durum: Bitti).
// What is typed after the sign narrows the list; ↑↓ move, ↵ chooses, Esc
// closes. The editor keeps those keys: the menu reads what they did - a
// new line after the query is ↵, the caret leaving the line is ↑ or ↓ - and
// takes it back.

type slashMenu struct {
	open  bool
	sign  string // "/" or "@"
	start int    // where the sign is
	sel   int
	at    int // the caret, last frame
}

// menuItem is a line of the menu.
type menuItem struct {
	title, hint, group string
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

// menuItems are the lines of the menu open, for what is typed after its sign.
func (a *app) menuItems(q string) []menuItem {
	var out []menuItem
	if a.slash.sign == "/" {
		for _, k := range slashItems(q) {
			k := k
			out = append(out, menuItem{title: k.name, hint: k.md, group: "Bloklar", run: func() { a.chooseSlash(k) }})
		}
		return out
	}
	start := a.slash.start
	tq := strings.TrimSpace(q)
	// dates
	today := a.clock()
	days := []namedDay{{"Bugün", today}, {"Yarın", today.AddDate(0, 0, 1)}, {"Dün", today.AddDate(0, 0, -1)}, {"Gelecek hafta", today.AddDate(0, 0, 7)}}
	if d, err := time.Parse("2006-01-02", tq); err == nil {
		days = append([]namedDay{{shortDate(d), d}}, days...)
	}
	for _, d := range days {
		if q != "" && match(q, d.name) < 0 && !strings.HasPrefix(d.d.Format("2006-01-02"), tq) {
			continue
		}
		ch := &chip{kind: "date", target: d.d.Format("2006-01-02")}
		out = append(out, menuItem{title: d.name, hint: shortDate(d.d), group: "Tarih", glyph: iconCalendar, run: func() { a.insertChip(start, ch) }})
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
		out = append(out, menuItem{title: title, hint: it.sub, group: "Sayfalar", glyph: iconPage, run: func() { a.insertChip(start, &chip{kind: "page", target: title}) }})
	}
	// typed, the pages first: a title is what is typed most
	if tq != "" {
		var pages, rest []menuItem
		for _, it := range out {
			if it.group == "Sayfalar" {
				pages = append(pages, it)
			} else {
				rest = append(rest, it)
			}
		}
		out = append(pages, rest...)
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
				out = append(out, menuItem{title: label, group: "Özellikler", glyph: iconSettings, run: func() {
					a.dropQuery(start)
					a.setValue(p, id, o)
				}})
			}
		}
	}
	// last, a page of what is typed, when none is called so
	if tq != "" && !exact && a.page != nil {
		out = append(out, menuItem{title: "Yeni sayfa: " + tq, group: "Yeni", glyph: iconPlus, run: func() {
			if _, err := a.page.Coll.NewPage(nil, tq); err != nil {
				a.saveErr = err.Error()
				return
			}
			a.insertChip(start, &chip{kind: "page", target: tq})
		}})
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
		ps, pe := text.ParagraphRange(caret - 1)
		if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-1).Code {
			return false
		}
		switch {
		case sign == "/" && ps == caret-1 && pe == caret:
		case sign == "@" && (ps == caret-1 || strings.ContainsAny(text.Slice(caret-2, caret-1).String(), " \t(")):
		default:
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
	case s.sign == "/" && strings.ContainsAny(q, " \t"):
		s.open = false
	case s.sign == "@" && (len([]rune(q)) > 40 || strings.HasSuffix(q, "  ")):
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
	t := a.doc.Text()
	_, pe := t.ParagraphRange(s.start)
	start := s.start
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, pe, richtext.Text{})
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
	ui.Overlay(c, func() {
		panel := ui.Column(c.Key("slash")).Absolute().Left(cr.X-4).Top(cr.Y+cr.H+6).Width(300).Padding(6).Gap(1).
			Radius(t.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label("Öneriler")
		t.Shadows.ShadowPop.Apply(panel)
		panel.Children(func() {
			if len(items) == 0 {
				ui.Text(c, "Eşleşen yok").FontSize(13).TextColor(cl.Muted).Padding(6, 10)
			}
			group := ""
			for i, it := range items {
				if it.group != group {
					group = it.group
					ui.Text(c.Key("g:"+group), group).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Padding(6, 10, 4, 10)
				}
				row := ui.Row(c.Key(i)).Height(34).Padding(0, 10).Gap(10).Radius(t.R(ryugo.RadiusControl) - 4).
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
					ui.Text(c, it.title).FontSize(14).TextColor(text).MaxLines(1).Shrink(1).Grow(1)
					if it.hint != "" {
						ui.Text(c, it.hint).FontSize(12.5).TextColor(muted).MaxLines(1).Shrink(1)
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
}
