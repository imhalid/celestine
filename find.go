package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// Finding in the page (⌘F): a bar over the title; every match in view is
// lit, the one being looked at more, and selected. ↵ goes to the next, ⇧↵
// back, Esc closes. Accents and case don't matter. Its second row replaces
// the match, or all of them.

type finder struct {
	open, replace bool
	query, with   string
	cur           int      // the match being looked at
	matches       [][2]int // the matches, rune ranges of the text
	ver           uint64   // the text and query they were found in
	q             string
	focus         bool // the field takes the focus next frame
}

func (a *app) openFind() {
	if a.page == nil || a.noPage() || a.doc == nil {
		return
	}
	f := &a.find
	f.open, f.focus = true, true
	if s, e := a.doc.Selection(); s != e { // what is selected is what is looked for
		if s > e {
			s, e = e, s
		}
		if sel := a.doc.Text().Slice(s, e).String(); !strings.Contains(sel, "\n") {
			f.query = sel
		}
	}
	f.ver = 0
}

// foldRunes is s for finding: a rune for each rune, lower case, Turkish
// letters as their ASCII.
func foldRunes(s string) []rune {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		f := []rune(fold(string(r)))
		if len(f) == 0 {
			f = []rune{r}
		}
		out = append(out, f[0])
	}
	return out
}

// findAll finds the query in the text, again when either changed.
func (a *app) findAll() {
	f := &a.find
	if f.ver == a.doc.Version() && f.q == f.query {
		return
	}
	queryChanged := f.q != f.query
	f.ver, f.q, f.matches = a.doc.Version(), f.query, nil
	q := foldRunes(f.query)
	if len(q) == 0 {
		return
	}
	text := foldRunes(a.doc.Text().String())
	for i := 0; i+len(q) <= len(text); i++ {
		if string(text[i:i+len(q)]) == string(q) {
			f.matches = append(f.matches, [2]int{i, i + len(q)})
			i += len(q) - 1
		}
	}
	if queryChanged { // the first match from the caret on
		_, caret := a.doc.Selection()
		f.cur = 0
		for i, m := range f.matches {
			if m[1] >= caret {
				f.cur = i
				break
			}
		}
		a.showMatch()
	}
	f.cur = min(f.cur, max(len(f.matches)-1, 0))
}

// showMatch selects the match being looked at.
func (a *app) showMatch() {
	f := &a.find
	if f.cur < len(f.matches) {
		m := f.matches[f.cur]
		a.doc.SetSelection(m[0], m[1])
	}
}

func (a *app) stepMatch(d int) {
	f := &a.find
	if n := len(f.matches); n > 0 {
		f.cur = (f.cur + d + n) % n
		a.showMatch()
	}
}

// replaceMatch replaces the match being looked at, or all of them.
func (a *app) replaceMatch(all bool) {
	f := &a.find
	if len(f.matches) == 0 {
		return
	}
	text := a.doc.Text()
	ms := f.matches[f.cur : f.cur+1]
	if all {
		ms = f.matches
	}
	a.doc.Edit(func(e *richtext.Edit) {
		for i := len(ms) - 1; i >= 0; i-- {
			m := ms[i]
			e.ReplaceString(m[0], m[1], f.with, text.AttributesAt(m[0]))
		}
	})
	f.ver = 0
	a.findAll()
	if !all {
		a.showMatch()
	}
}

// findBar is the bar over the title.
func (a *app) findBar(c *ui.Context) {
	f := &a.find
	if !f.open {
		return
	}
	a.findAll()
	t := ryugo.From(c)
	cl := t.Colors
	ui.Column(c.Key("find-bar")).Gap(6).Padding(8).Radius(rad(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).
		Margin(0, 0, 8, 0).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if a.iconButton(c, map[bool]*ui.SVG{true: iconChevronDown, false: iconChevron}[f.replace], "Değiştir’i aç/kapat") {
				f.replace = !f.replace
			}
			in := ryugo.Input(c.Key("find-in"), &f.query, ryugo.InputOptions{Label: "Sayfada bul", Placeholder: "Bul", PrefixIcon: iconSearch, Size: ryugo.Sm, Invalid: f.query != "" && len(f.matches) == 0})
			if f.focus {
				in.Focus()
				f.focus = false
			}
			switch {
			case in.Shortcut(0, ui.KeyEscape):
				a.closeFind()
				return
			case in.Submitted() && c.Modifiers()&ui.Shift != 0:
				a.stepMatch(-1)
			case in.Submitted():
				a.stepMatch(1)
			}
			count := "0/0"
			if len(f.matches) > 0 {
				count = strconv.Itoa(f.cur+1) + "/" + strconv.Itoa(len(f.matches))
			}
			ui.Text(c, count).FontSize(12.5).TextColor(cl.Muted).Width(48)
			if a.iconButton(c, iconBack, "Önceki (⇧↵)") {
				a.stepMatch(-1)
			}
			if a.iconButton(c, iconChevron, "Sonraki (↵)") {
				a.stepMatch(1)
			}
			if a.iconButton(c, iconClose, "Kapat (Esc)") {
				a.closeFind()
			}
		})
		if f.replace {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 0, 0, 32).Children(func() {
				in := ryugo.Input(c.Key("find-with"), &f.with, ryugo.InputOptions{Label: "Şununla değiştir", Placeholder: "Şununla değiştir", Size: ryugo.Sm})
				if in.Submitted() {
					a.replaceMatch(false)
				}
				if ryugo.Button(c.Key("replace-one"), "Değiştir", ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: len(f.matches) == 0}).Clicked() {
					a.replaceMatch(false)
				}
				if ryugo.Button(c.Key("replace-all"), "Tümünü değiştir", ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: len(f.matches) == 0}).Clicked() {
					a.replaceMatch(true)
				}
			})
		}
	})
}

func (a *app) closeFind() {
	a.find.open, a.find.matches = false, nil
}

// drawMatches lights the matches in view in the editor (DrawOver).
func (a *app) drawMatches(p *ui.Painter, r ui.Rect) {
	f := &a.find
	if !f.open || len(f.matches) == 0 {
		return
	}
	text := a.doc.Text()
	warn := ryugo.Painting(p).Colors.Warn
	for _, b := range a.shown(r) {
		s, e := text.ParagraphRange(b.Start)
		var in [][2]int
		cur := -1
		for i, m := range f.matches {
			if m[0] >= s && m[1] <= e {
				if i == f.cur {
					cur = len(in)
				}
				in = append(in, m)
			}
		}
		if len(in) == 0 {
			continue
		}
		var para richtext.Paragraph
		for _, pp := range text.Slice(s, e).Paragraphs() {
			para = pp
			break
		}
		para.Style = text.ParagraphStyleAt(s)
		l, left := layoutParagraph(para, r.W, a.font, a.theme)
		var line string
		for _, run := range para.Runs {
			line += run.Text
		}
		for i, m := range in {
			rects := l.SelectionRects(ui.TextInputRange{Start: units(line, m[0]-s), End: units(line, m[1]-s)})
			col := warn.Alpha(0.28)
			if i == cur {
				col = warn.Alpha(0.6)
			}
			for _, rc := range rects {
				p.Fill(ui.Rect{X: r.X + left + rc.X, Y: r.Y + b.Top + rc.Y, W: rc.W, H: rc.H}, col, rad(3))
			}
		}
	}
}

// units is where rune at is in line, in UTF-16 units, as layouts count.
func units(line string, at int) int {
	u, runes := 0, 0
	for _, r := range line {
		if runes >= at {
			break
		}
		runes++
		if r > 0xFFFF {
			u += 2
		} else {
			u++
		}
	}
	return u
}
