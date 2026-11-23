package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Finding in the page (⌘F): a bar over the title; every match in view is
// lit, the one being looked at more, and selected. ↵ goes to the next, ⇧↵
// back, Esc closes. Accents and case don't matter while the query has
// none: one with a capital or an accent is found as it is written (#212).
// Its second row replaces the match, or all of them.

type finder struct {
	open, replace bool
	query, with   string
	cur           int      // the match being looked at
	matches       [][2]int // the matches, rune ranges of the text
	hidden        int      // the matches inside folds, zooms and tables, which find does not go into
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
	seen := map[rune]rune{} // a letter is folded once a call, not each time it comes (#198)
	for _, r := range s {
		if r < 0x80 { // ASCII: its lower case, which fold gives it
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			out = append(out, r)
			continue
		}
		f, ok := seen[r]
		if !ok {
			fr := []rune(fold(string(r)))
			f = r
			if len(fr) > 0 {
				f = fr[0]
			}
			seen[r] = f
		}
		out = append(out, f)
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
	f.ver, f.q, f.matches, f.hidden = a.doc.Version(), f.query, nil, 0
	exact := findExact(f.query)
	q := findRunes(f.query, exact)
	if len(q) == 0 {
		return
	}
	text := a.doc.Text()
	for _, m := range findIn(findRunes(text.String(), exact), q) {
		f.matches = append(f.matches, m)
	}
	f.hidden = findHidden(text, q, exact)
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

// findExact says whether query is found as it is written: when it has a capital or a letter with a mark, as
// "Ön" is not "on", and a replace takes only what is written so (#212). Lower case and without marks, it finds
// every way of writing it.
func findExact(query string) bool { return string(foldRunes(query)) != query }

// findRunes is s as find compares it: folded, or as it is when the query is exact; a rune for each rune.
func findRunes(s string, exact bool) []rune {
	if exact {
		return []rune(s)
	}
	return foldRunes(s)
}

// findIn are the places of q in text, one after the other, not overlapping.
func findIn(text, q []rune) [][2]int {
	var out [][2]int
	for i := 0; i+len(q) <= len(text); i++ {
		if text[i] == q[0] && slices.Equal(text[i:i+len(q)], q) { // the runes compared, not made into strings at each place (#198)
			out = append(out, [2]int{i, i + len(q)})
			i += len(q) - 1
		}
	}
	return out
}

// findHidden counts the matches of q that find does not go into: in what a fold or a zoom holds, folds in it
// too, and in a table's cells. The bar says how many there are, so that a replace of all does not seem to
// have taken every one (#212).
func findHidden(t richtext.Text, q []rune, exact bool) int {
	n := 0
	for _, r := range t.Runs(0, t.Len()) {
		att := r.Attributes.Attachment
		if att == nil {
			continue
		}
		ch, ok := att.Data.(*chip)
		switch {
		case !ok:
		case (ch.kind == "fold" || ch.kind == "zoom") && ch.hidden != nil:
			n += len(findIn(findRunes(ch.hidden.String(), exact), q)) + findHidden(*ch.hidden, q, exact)
		case ch.kind == "table" && ch.tbl != nil:
			for _, row := range ch.tbl.rows {
				for _, cell := range row {
					n += len(findIn(findRunes(cell, exact), q))
				}
			}
		}
	}
	return n
}

// showMatch selects the match being looked at.
func (a *app) showMatch() {
	f := &a.find
	if f.cur < len(f.matches) {
		m := f.matches[f.cur]
		a.doc.SetSelection(m[0], m[1])
		a.revealWant = true
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

// findBar is the bar that drops from the top right of the page, over it: the page under it stays where it is.
func (a *app) findBar(c *ui.Context) {
	f := &a.find
	if !f.open {
		return
	}
	a.findAll()
	t := ryugo.From(c)
	cl := t.Colors
	w := float32(440)
	if a.pageR.W > 0 {
		w = min(w, a.pageR.W-32)
	}
	bar := ui.Column(c.Key("find-bar")).Absolute().Top(a.barInset+8).Right(18).Width(w).Gap(6).Padding(8).
		Radius(rad(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline)
	bar.Transition(findDrop) // it comes down from above, in a breath, and goes up again
	bar.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if a.iconButton(c, map[bool]*ui.SVG{true: iconChevronDown, false: iconChevron}[f.replace], locales.FindToggleReplace()) {
				f.replace = !f.replace
			}
			in := a.fieldMenu(ryugo.Input(c.Key("find-in"), &f.query, ryugo.InputOptions{Label: locales.FindFindInPage(), Placeholder: locales.FindPlaceholder(), PrefixIcon: iconSearch, Size: ryugo.Sm, Invalid: f.query != "" && len(f.matches) == 0}), &f.query, fieldOpts{oneLine: true})
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
			if a.iconButton(c, iconBack, locales.FindPrevious()) {
				a.stepMatch(-1)
			}
			if a.iconButton(c, iconChevron, locales.FindNext()) {
				a.stepMatch(1)
			}
			if a.iconButton(c, iconClose, locales.FindClose()) {
				a.closeFind()
			}
		})
		if f.hidden > 0 {
			ui.Text(c, locales.FindHiddenMore(f.hidden)).FontSize(12).TextColor(cl.Muted).Padding(0, 0, 0, 32)
		}
		if f.replace {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 0, 0, 32).Children(func() {
				in := a.fieldMenu(ryugo.Input(c.Key("find-with"), &f.with, ryugo.InputOptions{Label: locales.QueryReplaceWith(), Placeholder: locales.QueryReplaceWith(), Size: ryugo.Sm}), &f.with, fieldOpts{oneLine: true})
				if in.Submitted() {
					a.replaceMatch(false)
				}
				if ryugo.Button(c.Key("replace-one"), locales.CommonChange(), ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: len(f.matches) == 0}).Clicked() {
					a.replaceMatch(false)
				}
				if ryugo.Button(c.Key("replace-all"), locales.QueryReplaceAll(), ryugo.ButtonOptions{Size: ryugo.Sm, Disabled: len(f.matches) == 0}).Clicked() {
					a.replaceMatch(true)
				}
			})
		}
	})
}

// findDrop is the find bar's coming and going: down from above, up and out.
var findDrop = ui.ElementTransition{
	Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond}},
	Enter:     &ui.Motion{Opacity: 0, Y: -18},
	Exit:      &ui.Motion{Opacity: 0, Y: -18},
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
