package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// The differences of a copy from the page now, in the history itself and side by side: the toolbar's button turns
// the stack into one card of two columns, the copy on the left and the page now on the right, a line of one beside
// the line of the other it became. What only the copy has is red, what only the page has green, the words that
// differ marked within a pair. The timeline and the arrows go on choosing the copy.

// tmDiffRow is a row of the differences: the copy's line and the page's, either missing where only the other has
// one; or a run of lines both have, left out.
type tmDiffRow struct {
	left, right *DiffOp
	same        int // not a line: this many lines both have, left out
}

// tmDiffRows sets the copy's text beside the page's: the lines both have on both sides (a long run of them cut to
// the few at its edges), and in each stretch that differs the copy's lines paired with the page's, the first with
// the first.
func tmDiffRows(old, now string) []tmDiffRow {
	lines := func(s string) []string { return strings.Split(strings.TrimRight(s, "\n"), "\n") }
	ops := DiffLines(lines(old), lines(now))
	PairWords(ops)
	const edge = 2
	var out []tmDiffRow
	for i := 0; i < len(ops); {
		j := i
		if ops[i].Kind == '=' {
			for j < len(ops) && ops[j].Kind == '=' {
				j++
			}
			for k := i; k < j; k++ {
				if j-i > 2*edge+2 && k == i+edge {
					out = append(out, tmDiffRow{same: j - i - 2*edge})
					k = j - edge
				}
				out = append(out, tmDiffRow{left: &ops[k], right: &ops[k]})
			}
			i = j
			continue
		}
		var was, is []*DiffOp // the copy's lines and the page's, in this stretch
		for ; j < len(ops) && ops[j].Kind != '='; j++ {
			if ops[j].Kind == '+' {
				was = append(was, &ops[j])
			} else {
				is = append(is, &ops[j])
			}
		}
		for k := 0; k < max(len(was), len(is)); k++ {
			var r tmDiffRow
			if k < len(was) {
				r.left = was[k]
			}
			if k < len(is) {
				r.right = is[k]
			}
			out = append(out, r)
		}
		i = j
	}
	return out
}

// tmDiff are the rows of the differences of the copy at place i from the page now.
func (a *app) tmDiff(i int) []tmDiffRow {
	t := &a.tm
	if rows, ok := t.diffs[i]; ok {
		return rows
	}
	rows := tmDiffRows(a.tmText(i), a.tmText(0))
	if t.diffs == nil {
		t.diffs = map[int][]tmDiffRow{}
	}
	t.diffs[i] = rows
	return rows
}

// tmDiffView draws the differences of the copy at the front in the stack's room.
func (a *app) tmDiffView(c *ui.Context, ox, aw, wh float32) {
	t := &a.tm
	th := ryugo.From(c)
	cl := th.Colors
	if t.diffFor != t.sel { // another copy: from its top
		t.diffScroll, t.diffFor = ui.ScrollState{}, t.sel
	}
	rows := a.tmDiff(t.sel)
	w := min(aw-tmBarsWidth()-48, float32(1100))
	top := a.headerHeight(c) + 8
	half := (w - 1) / 2
	card := ui.Column(c.Key("tm-diff-card")).Absolute().Left(ox+(aw-tmBarsWidth()-w)/2).Top(top).Size(w, wh-tmFootH-top-8).Translate(0, (1-t.in)*28).
		Background(a.bg(c)).Border(1, cl.Hairline).Radius(th.R(ryugo.RadiusPop)).Clip().Opacity(t.in).Label(T("Differences"))
	th.Shadows.ShadowPop.Apply(card)
	card.Children(func() {
		ui.Row(c.Key("tm-diff-head")).FillWidth().Height(38).AlignItems(ui.Center).Children(func() {
			for k, s := range []string{a.tmWhen(t.sel), T("Now")} {
				ui.Text(c.Key("tm-diff-side:"+itoa(k)), s).Width(half).Padding(0, 16).FontSize(13).FontWeight(650).TextColor(cl.Text2).MaxLines(1)
				if k == 0 {
					ui.Box(c).Width(1).Height(38).Background(cl.Hairline)
				}
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
		differs := false
		for _, r := range rows {
			differs = differs || r.same == 0 && r.left != r.right
		}
		if !differs {
			ui.Text(c.Key("tm-diff-none"), T("This copy is the same as the page now.")).FillWidth().Padding(28, 16).TextAlign(ui.Center).
				FontSize(13).TextColor(cl.Muted)
			return
		}
		a.scrolled(c, "tm-diff", &t.diffScroll, ui.Scroll(c.Key("tm-diff-rows")).FillWidth().Grow(1).MinHeight(0), func() {
			ui.Column(c).FillWidth().Padding(6, 0, 12, 0).Children(func() {
				for i, r := range rows {
					if r.same > 0 {
						ui.Text(c.Key("tm-same:"+itoa(i)), Tf("⋯ %d lines the same", r.same)).FillWidth().Padding(5, 16).TextAlign(ui.Center).
							FontSize(11.5).TextColor(cl.Muted.Alpha(0.8))
						continue
					}
					ui.Row(c.Key("tm-dr:" + itoa(i))).FillWidth().AlignItems(ui.Stretch).Children(func() {
						a.tmDiffCell(c, cl, r.left, r.left != r.right, cl.Up, half)
						ui.Box(c).Width(1).Background(cl.Hairline)
						a.tmDiffCell(c, cl, r.right, r.left != r.right, cl.Down, half)
					})
				}
			})
		})
	})
}

// tmDiffCell draws one side of a row of the differences: the line, tinted where the other side has not the same,
// the words that differ marked; or the room of a line only the other side has.
func (a *app) tmDiffCell(c *ui.Context, cl ryugo.Colors, o *DiffOp, differs bool, tint ui.Color, w float32) {
	cell := ui.Box(c).Width(w).Padding(3, 16)
	if o == nil {
		cell.Background(cl.Text.Alpha(0.035))
		return
	}
	if differs {
		cell.Background(tint.Alpha(0.12))
	}
	cell.Children(func() {
		if differs && len(o.Words) > 0 {
			var spans []ui.Span
			for k, wd := range o.Words {
				if k > 0 {
					spans = append(spans, ui.Span{Text: " ", Size: 13.5})
				}
				sp := ui.Span{Text: wd.Text, Size: 13.5, Color: cl.Text}
				if wd.Kind != '=' {
					sp.Weight, sp.Background, sp.Radius = 650, tint.Alpha(0.28), 3
				}
				spans = append(spans, sp)
			}
			ui.RichText(c, spans...).FillWidth()
			return
		}
		text := o.Text
		if strings.TrimSpace(text) == "" {
			text = " "
		}
		ink := cl.Text
		if !differs {
			ink = cl.Text2
		}
		ui.Text(c, text).FontSize(13.5).TextColor(ink).FillWidth().Wrap()
	})
}
