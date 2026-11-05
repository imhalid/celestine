package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The / and @ menu, under the caret, as Thymer's: one menu, whichever sign
// opens it, at the start of a word.
//   - Before anything is typed: the four kinds of line used last
//     (Öneriler), then the menus: Satırı biçimlendir…, Ekle…, Tarih ya da
//     saat, Özellik…, Bağlantı…; a ↵ or a click goes into one.
//   - A menu has its own list - the lines to turn into, a calendar and the
//     days by name, the pages to link, the page's choices - and what is
//     typed narrows it, shown in a field at its top; Esc, or ← Geri, goes
//     back.
//   - Typed in the first menu, the sign's text narrows everything at once.
// What is typed after the sign is in the text, the caret in the line; ↑↓ move, ↵ chooses, Esc closes. The editor keeps those keys: the menu reads what they did - a
// new line after the query is ↵, the caret leaving the line is ↑ or ↓ - and
// takes it back.

type slashMenu struct {
	open   bool
	sign   string // "/" or "@"
	start  int    // where the sign is
	sel    int
	seen   int    // the choice the list was last scrolled to show
	at     int    // the caret, last frame
	view   string // the menu gone into: "format", "insert", "date", "link", "prop"; "" the first
	field  string // what is typed in a menu's own field (date, link)
	fseen  string // the field, last frame
	fin    bool   // the field has had the focus
	cal    time.Time
	scroll ui.ScrollState
}

// menuItem is a line of the menu.
type menuItem struct {
	title, hint, group string
	sub                string // faint, under the title
	nav                bool   // goes into a menu: the menu stays open
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

// The kinds of line: those a line is turned into, and those put in.
var (
	formatKinds   = map[string]bool{"text": true, "h1": true, "h2": true, "h3": true, "bullets": true, "numbers": true, "tasks": true, "quote": true}
	defaultRecent = []string{"tasks", "h1", "h2", "bullets"}
)

// kindIcon is the icon of each kind of line.
var kindIcon = map[string]*ui.SVG{
	"text":    icon(`<path d="M5 7h14M5 12h14M5 17h9"/>`),
	"h1":      icon(`<path d="M4.5 6v12M4.5 12h8M12.5 6v12"/><path d="m16.5 10 2-1.2V17"/>`),
	"h2":      icon(`<path d="M4.5 6v12M4.5 12h8M12.5 6v12"/><path d="M16 10.5a1.8 1.8 0 1 1 3.1 1.2L16 16.5h4"/>`),
	"h3":      icon(`<path d="M4.5 6v12M4.5 12h8M12.5 6v12"/><path d="M16 9.5h3.5L17.5 12a1.9 1.9 0 1 1-1.6 3"/>`),
	"bullets": icon(`<circle cx="5.5" cy="7" r="1"/><circle cx="5.5" cy="12" r="1"/><circle cx="5.5" cy="17" r="1"/><path d="M10 7h9M10 12h9M10 17h9"/>`),
	"numbers": icon(`<path d="M10 7h9M10 12h9M10 17h9"/><path d="m4.5 6 1.5-1v5M4.5 14.5h3l-3 3.5h3"/>`),
	"tasks":   icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="m8.5 12 2.5 2.5 4.5-5"/>`),
	"quote":   icon(`<path d="M5 6v12"/><path d="M9.5 8h9M9.5 12h9M9.5 16h5"/>`),
	"code":    icon(`<path d="m8.5 8-4 4 4 4M15.5 8l4 4-4 4"/>`),
	"hr":      icon(`<path d="M4 12h3M10.5 12h3M17 12h3"/>`),
}

var (
	iconFormat = icon(`<path d="M5 6h14M5 11h14M5 16h8"/><path d="M17 15v5M14.5 17.5h5"/>`)
	iconNone   = icon(`<circle cx="11" cy="11" r="6"/><path d="m20 20-4.5-4.5M8.5 11h5"/>`)
	iconInsert = icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="3"/><path d="M12 8.5v7M8.5 12h7"/>`)
)

// recentKinds are the four kinds of line used last, the usual ones filling
// what has not been used.
func (a *app) recentKinds() []string {
	out := append([]string{}, a.settings.Recent...)
	for _, k := range defaultRecent {
		if len(out) >= 4 {
			break
		}
		if !contains(out, k) {
			out = append(out, k)
		}
	}
	return out[:min(len(out), 4)]
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// useKind notes a kind of line as the one used last.
func (a *app) useKind(kind string) {
	r := []string{kind}
	for _, k := range a.settings.Recent {
		if k != kind {
			r = append(r, k)
		}
	}
	a.settings.Recent = r[:min(len(r), 4)]
	a.saveSettings()
}

func kindByID(id string) (struct{ kind, name, md string }, bool) {
	for _, k := range blockKinds {
		if k.kind == id {
			return k, true
		}
	}
	return struct{ kind, name, md string }{}, false
}

// enterView goes into one of the menus.
func (a *app) enterView(v string) {
	s := &a.slash
	if hasField(s.view) { // the field goes: the editor takes the focus back
		a.refocus = true
	}
	s.view, s.sel, s.seen, s.scroll = v, 0, -1, ui.ScrollState{}
	s.field, s.fseen, s.fin = "", "", false
	s.cal = a.clock()
}

// menuItems are the lines of the menu open, for what is typed after its sign.
func (a *app) menuItems(q string) []menuItem {
	s := &a.slash
	switch {
	case s.view != "":
		return a.viewItems(s.view, q)
	case q != "":
		return a.searchItems(q)
	}
	var out []menuItem
	for _, id := range a.recentKinds() {
		if k, ok := kindByID(id); ok {
			out = append(out, menuItem{title: k.name, glyph: kindIcon[k.kind], run: func() { a.chooseSlash(k) }})
		}
	}
	nav := func(title, sub, view string, glyph *ui.SVG) {
		out = append(out, menuItem{title: title, sub: sub, group: "~", glyph: glyph, nav: true, run: func() { a.enterView(view) }})
	}
	nav("Satırı biçimlendir…", "başlıklar, listeler, alıntılar", "format", iconFormat)
	nav("Ekle…", "kod, ayırıcı", "insert", iconInsert)
	nav("Tarih ya da saat", "", "date", iconCalendar)
	if len(a.choiceItems("")) > 0 {
		nav("Özellik…", "sayfanın seçimleri", "prop", iconSettings)
	}
	nav("Bağlantı…", "sayfalar", "link", iconPage)
	return out
}

// choiceItems are the values of the page's choices, for q.
func (a *app) choiceItems(q string) []menuItem {
	var out []menuItem
	p := a.page
	if p == nil {
		return nil
	}
	start := a.slash.start
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
			out = append(out, menuItem{title: label, glyph: iconSettings, run: func() {
				a.dropQuery(start)
				a.setValue(p, id, o)
			}})
		}
	}
	return out
}

// viewItems are the lines of one of the menus, for q.
func (a *app) viewItems(view, q string) []menuItem {
	start := a.slash.start
	tq := strings.TrimSpace(q)
	var out []menuItem
	switch view {
	case "format", "insert":
		for _, k := range slashItems(q) {
			k := k
			if formatKinds[k.kind] == (view == "format") {
				out = append(out, menuItem{title: k.name, glyph: kindIcon[k.kind], run: func() { a.chooseSlash(k) }})
			}
		}
	case "date":
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
			out = append(out, menuItem{title: d.name, hint: shortDate(d.d), glyph: iconCalendar, run: func() { a.insertChip(start, ch) }})
		}
	case "link":
		exact, n := false, 0
		for _, it := range a.places() {
			if it.to.kind != "page" || (a.page != nil && it.to.dir == a.page.Dir) || (q != "" && match(q, it.title) < 0) {
				continue
			}
			if strings.EqualFold(it.title, tq) {
				exact = true
			}
			if n++; n > 40 {
				continue
			}
			title := it.title
			out = append(out, menuItem{title: title, sub: it.sub, glyph: iconPage, run: func() { a.insertChip(start, &chip{kind: "page", target: title}) }})
		}
		if tq != "" && !exact && a.page != nil {
			out = append(out, menuItem{title: "Yeni sayfa: " + tq, glyph: iconPlus, run: func() {
				if _, err := a.page.Coll.NewPage(nil, tq); err != nil {
					a.saveErr = err.Error()
					return
				}
				a.insertChip(start, &chip{kind: "page", target: tq})
			}})
		}
	case "prop":
		out = a.choiceItems(q)
	}
	return out
}

// searchItems are the lines of the first menu for what is typed after its
// sign: everything that matches, grouped.
func (a *app) searchItems(q string) []menuItem {
	start := a.slash.start
	tq := strings.TrimSpace(q)
	var kinds, days, pages, props, fresh []menuItem
	// the kinds of line
	for _, k := range slashItems(q) {
		k := k
		kinds = append(kinds, menuItem{title: k.name, group: "Satırı biçimlendir", glyph: kindIcon[k.kind], run: func() { a.chooseSlash(k) }})
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
	if a.slash.sign == "@" {
		add(pages, days, props, kinds, fresh)
	} else {
		add(kinds, days, props, pages, fresh)
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
	if hasField(s.view) { // typed in the menu's field, not in the text
		return s.field
	}
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
	if hasField(s.view) { // the focus is in the field: the text is left alone
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
		s.open = false
		if len(items) > 0 {
			it := items[min(s.sel, len(items)-1)]
			it.run()
			s.open = it.nav
		}
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
	a.useKind(k.kind)
	s.open = false
}

// The menu's rows have fixed heights, so its height is known before it is
// laid out, and it can be put above the caret when there is no room below.
const (
	menuRow   = 34
	menuRow2  = 46 // a row with a second line
	menuHead  = 26
	menuSep   = 9
	menuTop   = 32 // the header: the title, or ← Geri
	menuIn    = 38 // the field of a menu
	menuCal   = 224
	menuEmpty = 112 // the menu with nothing in it
	menuPad   = 6
	menuW     = 300
	menuGap   = 6
)

// rowH is the height of a row.
func menuRowH(it menuItem) float32 {
	if it.sub != "" {
		return menuRow2
	}
	return menuRow
}

// menuRows places each row of the menu in its content: where it starts and
// ends, and the content's height; extra is what is under the rows.
func menuRows(items []menuItem, extra float32) (tops, ends []float32, total float32) {
	group := ""
	for i, it := range items {
		if i > 0 {
			total++
		}
		if it.group != group {
			group = it.group
			switch group {
			case "":
			case "~":
				total += menuSep + 1
			default:
				total += menuHead + 1
			}
		}
		tops, ends = append(tops, total), append(ends, total+menuRowH(it))
		total += menuRowH(it)
	}
	if len(items) == 0 {
		total = menuEmpty
	}
	return tops, ends, total + extra
}

// hasField is whether the menu has a field at its top, for what is typed.
func hasField(view string) bool { return view == "date" || view == "link" }

var viewTitle = map[string]string{"format": "Satırı biçimlendir", "insert": "Ekle", "date": "Tarih ya da saat", "link": "Bağlantı", "prop": "Özellik"}

// slashView is the menu under the caret.
func (a *app) slashView(c *ui.Context) {
	s := &a.slash
	if !s.open || a.doc == nil || a.coll != nil || a.trashOpen {
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		if s.view != "" {
			a.enterView("")
		} else {
			s.open = false
		}
		return
	}
	if a.refocus {
		c.Invalidate() // a frame in which the editor takes the focus
	}
	if hasField(s.view) && s.fin && a.focused { // a click in the text
		s.open = false
		return
	}
	q := a.slashQuery()
	items := a.menuItems(q)
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
	extra := float32(0)
	if s.view == "date" {
		extra = menuCal + 3 // the gaps, a divider and the calendar
		if len(items) == 0 {
			extra = menuCal + 1
		}
	}
	head := float32(menuTop + 1) // and the divider under it
	if hasField(s.view) {
		head += menuIn + 1
	}
	ww, wh := c.Size()
	tops, ends, total := menuRows(items, extra)
	chrome := float32(2*menuPad+2) + head // the padding, the border and the header
	below := wh - (cr.Y + cr.H + menuGap) - 8 - chrome
	above := cr.Y - menuGap - 8 - chrome
	inner, top := min(total, below), cr.Y+cr.H+menuGap
	if total > below && above > below {
		inner = min(total, above)
		top = cr.Y - menuGap - inner - chrome
	}
	inner = max(inner, menuRow)
	if s.fseen != s.field { // typed: the first of the list again
		s.sel, s.seen, s.scroll.Y, s.fseen = 0, -1, 0, s.field
	}
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
	choose := func(it menuItem) {
		a.doc.SetSelection(s.at, s.at)
		s.open = false
		it.run()
		s.open = it.nav
	}
	ui.Overlay(c, func() {
		panel := ui.Column(c.Key("slash")).Absolute().Left(left).Top(top).Width(menuW).Padding(menuPad).
			Radius(t.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label("Öneriler")
		t.Shadows.ShadowPop.Apply(panel)
		panel.Children(func() {
			// the header: the title of the menu and the sign, or the way back
			ui.Row(c.Key("slash-head")).Height(menuTop).Padding(0, 6).AlignItems(ui.Center).Gap(8).Children(func() {
				if s.view == "" {
					title := "Öneriler"
					if q != "" {
						title = "Sonuçlar"
					}
					ui.Text(c, title).FontSize(12.5).TextColor(cl.Muted).Grow(1)
					ui.Text(c, "bu menü").FontSize(11.5).TextColor(cl.Muted)
					ui.Box(c.Key("sign")).Padding(1, 7).Radius(5).Background(cl.Fill).Children(func() {
						ui.Text(c, s.sign).FontSize(12.5).FontWeight(650).TextColor(cl.Accent)
					})
					return
				}
				back := ui.Row(c.Key("slash-back")).Gap(6).AlignItems(ui.Center).Padding(4, 6).Radius(6).Role(ui.RoleButton).Label("Geri").Cursor(ui.CursorPointer)
				if back.Hovered() {
					back.Background(cl.Fill)
				}
				back.Children(func() {
					ui.Icon(c, iconBack).Width(14).Height(14).TextColor(cl.Muted)
					ui.Text(c, "Geri").FontSize(13).TextColor(cl.Muted)
				})
				if back.Clicked() {
					a.doc.SetSelection(s.at, s.at)
					a.enterView("")
				}
				ui.Box(c.Key("slash-gap")).Grow(1)
				ui.Text(c, viewTitle[s.view]).FontSize(12.5).TextColor(cl.Muted)
			})
			ui.Box(c.Key("slash-d1")).Height(1).Background(cl.Hairline).Margin(0, -menuPad)
			if hasField(s.view) { // what is typed: a field of its own, the focus in it
				ui.Row(c.Key("slash-field")).Height(menuIn).Padding(0, 6).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconSearch).Width(15).Height(15).TextColor(cl.Muted)
					ph := map[string]string{"link": "Sayfa ara…", "date": "Yarın, 2026-11-10…"}[s.view]
					in := ui.TextInputBase(c.Key("slash-in"), &s.field).Label(ph).Placeholder(ph).PlaceholderColor(cl.Muted).
						FontSize(14).Grow(1).FocusRing(false).AutoFocus()
					if in.Focused() {
						s.fin = true
					}
					switch {
					case in.Shortcut(0, ui.KeyEscape):
						a.enterView("")
						a.doc.SetSelection(s.at, s.at)
					case in.Shortcut(0, ui.KeyDown):
						s.sel = min(s.sel+1, len(items)-1)
					case in.Shortcut(0, ui.KeyUp):
						s.sel = max(s.sel-1, 0)
					case in.Submitted() && len(items) > 0:
						choose(items[max(0, min(s.sel, len(items)-1))])
					}
				})
				ui.Box(c.Key("slash-d2")).Height(1).Background(cl.Hairline).Margin(0, -menuPad)
			}
			ui.Scroll(c).TrackScroll(&s.scroll).Height(inner).Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					if len(items) == 0 {
						ui.Column(c.Key("empty")).Height(menuEmpty).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Icon(c, iconNone).Width(30).Height(30).TextColor(cl.Muted.Alpha(0.6))
							none := map[string]string{"link": "Eşleşen sayfa yok", "date": "Eşleşen gün yok"}[s.view]
							if none == "" {
								none = "Eşleşen yok"
							}
							ui.Text(c, none).FontSize(13).TextColor(cl.Muted)
						})
					}
					group := ""
					for i, it := range items {
						if it.group != group {
							group = it.group
							switch group {
							case "":
							case "~":
								ui.Column(c.Key("sep")).Height(menuSep).Padding(0, 10).Justify(ui.Center).Children(func() {
									ui.Box(c).Height(1).Background(cl.Hairline)
								})
							default:
								ui.Text(c.Key("g:"+group), group).FontSize(11.5).FontWeight(650).TextColor(cl.Muted).
									Height(menuHead).Padding(8, 10, 0, 10)
							}
						}
						row := ui.Row(c.Key(i)).Height(menuRowH(it)).Padding(0, 10).Gap(10).Radius(t.R(ryugo.RadiusControl) - 4).
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
							if it.nav {
								ui.Icon(c, iconChevron).Width(14).Height(14).TextColor(muted)
							}
						})
						if row.Clicked() {
							choose(it)
						}
					}
					if s.view == "date" { // the calendar, under the days named
						if len(items) > 0 {
							ui.Box(c.Key("cal-d")).Height(1).Background(cl.Hairline)
						}
						ui.Column(c.Key("cal")).Height(menuCal).Padding(4, 2).AlignItems(ui.Center).Children(func() {
							if cal := ui.Calendar(c, &s.cal); cal.Changed() {
								choose(menuItem{run: func() {
									a.insertChip(s.start, &chip{kind: "date", target: s.cal.Format("2006-01-02")})
								}})
							}
						})
					}
				})
			})
		})
	})
}
