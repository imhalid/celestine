package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
)

// The / menu: "/" typed at the start of a line opens a list of the kinds
// of block under the caret; what is typed after it narrows the list, ↑↓
// move, ↵ turns the line into the one chosen (the "/…" goes), Esc or a
// space closes it. The editor keeps the keys: the menu reads what they did
// - a new line after "/…" is ↵, the caret leaving the line is ↑ or ↓ - and
// takes it back.

type slashMenu struct {
	open  bool
	start int // where the "/" is
	sel   int
	at    int // the caret, last frame
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

// slashQuery is what is typed after the "/".
func (a *app) slashQuery() string {
	s := &a.slash
	_, caret := a.doc.Selection()
	t := a.doc.Text()
	if caret <= s.start || caret > t.Len() {
		return ""
	}
	return t.Slice(s.start+1, caret).String()
}

// trackSlash opens, follows and closes the menu, after an edit or a move
// of the caret; it reports whether it changed the text.
func (a *app) trackSlash(text richtext.Text, caret int, edited, typed bool) bool {
	s := &a.slash
	if !s.open {
		if typed && caret > 0 && text.Slice(caret-1, caret).String() == "/" {
			ps, pe := text.ParagraphRange(caret - 1)
			if ps == caret-1 && pe == caret && !text.ParagraphStyleAt(ps).CodeBlock {
				*s = slashMenu{open: true, start: caret - 1, at: caret}
			}
		}
		return false
	}
	if s.start >= text.Len() || text.Slice(s.start, s.start+1).String() != "/" {
		s.open = false
		return false
	}
	ps, pe := text.ParagraphRange(s.start)
	items := slashItems(a.slashQueryAt(text, caret))
	switch {
	case edited && caret > pe && pe < text.Len() && caret == pe+1:
		// ↵: the line was broken after "/…"; it is joined back, and the
		// choice made
		a.doc.Edit(func(ed *richtext.Edit) { ed.Replace(pe, pe+1, richtext.Text{}) })
		if len(items) > 0 {
			a.chooseSlash(items[min(s.sel, len(items)-1)])
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
	case strings.ContainsAny(a.slashQueryAt(text, caret), " \t"):
		s.open = false
	case len(items) == 0 && caret-s.start > 4:
		s.open = false
	}
	s.at = caret
	return false
}

func (a *app) slashQueryAt(text richtext.Text, caret int) string {
	s := &a.slash
	if caret <= s.start+1 {
		return ""
	}
	_, pe := text.ParagraphRange(s.start)
	return text.Slice(s.start+1, min(caret, pe)).String()
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
	items := slashItems(a.slashQuery())
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
		panel := ui.Column(c.Key("slash")).Absolute().Left(cr.X-4).Top(cr.Y+cr.H+6).Width(280).Padding(6).Gap(1).
			Radius(t.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label("Blok menüsü")
		t.Shadows.ShadowPop.Apply(panel)
		panel.Children(func() {
			ui.Text(c, "Bloklar").FontSize(11.5).FontWeight(650).TextColor(cl.Muted).Padding(6, 10, 4, 10)
			if len(items) == 0 {
				ui.Text(c, "Eşleşen yok").FontSize(13).TextColor(cl.Muted).Padding(6, 10)
			}
			for i, k := range items {
				row := ui.Row(c.Key("slash:"+k.kind)).Height(34).Padding(0, 10).Gap(10).Radius(t.R(ryugo.RadiusControl) - 4).
					AlignItems(ui.Center).Label(k.name).Role(ui.RoleButton)
				text, muted := cl.Text, cl.Muted
				if i == s.sel {
					row.Background(cl.Accent)
					text, muted = cl.OnAccent, cl.OnAccent.Alpha(0.75)
				} else if row.Hovered() {
					row.Background(cl.Fill)
				}
				row.Children(func() {
					ui.Text(c, k.name).FontSize(14).TextColor(text).Grow(1)
					ui.Text(c, k.md).FontSize(12.5).Font(ryugo.FontMono).TextColor(muted)
				})
				if row.Clicked() {
					a.chooseSlash(k)
				}
			}
		})
	})
}
