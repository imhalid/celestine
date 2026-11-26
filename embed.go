package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Embeds: a line of ![[Page]] by itself shows that page's text in a card, as
// the page is now: when it changes, the card does. The line is what the file
// has; the card opens the page when it is clicked. It is a chip of its own
// kind (chips.go), laid out as a block of a fixed height: the first lines of
// the page, faded at the foot.

const embedH = 200

// embedAttachment is the card of an embed.
func (a *app) embedAttachment(ch *chip) *richtext.Attachment {
	return &richtext.Attachment{Width: a.blockWidth(), Height: embedH, Name: locales.BlocksEmbeddedPage(ch.target), Data: ch,
		Draw: func(p *ui.Painter, r ui.Rect) { a.drawEmbed(p, r, ch) }}
}

// embedLines are the first lines of the page's text, as Markdown, for a
// card; read again when the page changed.
func (a *app) embedLines(p *Page) []string {
	key := p.Dir + "|" + p.Modified.Format("150405.000000000")
	if v, ok := a.embedCache[key]; ok {
		return v
	}
	if a.embedCache == nil || len(a.embedCache) > 300 {
		a.embedCache = map[string][]string{}
	}
	var out []string
	if body, err := p.Text(); err == nil {
		inFence := false
		for _, ln := range strings.Split(body, "\n") {
			if fenceLine.MatchString(ln) {
				inFence = !inFence
				continue
			}
			t := strings.TrimSpace(ln)
			if t == "" || t == "---" || listMarkLine.MatchString(ln) { // a list's look, which no reader shows (listmd.go)
				continue
			}
			if m := h1Line.FindStringSubmatch(ln); m != nil && len(out) == 0 && strings.EqualFold(strings.TrimSpace(m[1]), p.Title) {
				continue // the title is above
			}
			if inFence {
				ln = "`" + t + "`"
			}
			out = append(out, ln)
			if len(out) == 14 {
				break
			}
		}
	}
	a.embedCache[key] = out
	return out
}

// drawEmbed draws the card: the page's name, and the first lines of its text.
func (a *app) drawEmbed(p *ui.Painter, r ui.Rect, ch *chip) {
	t := ryugo.Painting(p)
	cl := t.Colors
	th := p.Theme()
	r = ui.Rect{X: r.X, Y: r.Y + 3, W: r.W, H: r.H - 6}
	a.chipHits = append(a.chipHits, chipHit{r, ch})
	target := a.pageTitled(ch.target)
	hover := a.hoverCh == ch
	p.Fill(r, cl.WellBorder, rad(12))
	p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, map[bool]ui.Color{true: cl.Fill, false: cl.Well}[hover], rad(11))
	p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 10, W: 3, H: r.H - 20}, cl.Accent.Alpha(0.8), rad(1.5))
	size := a.fontSize() * 0.92
	head := ui.Span{Text: ch.target, Size: size, Weight: 680, Color: cl.Accent}
	if target == nil {
		head.Color = cl.Muted
		head.Text = locales.BlocksEmbedMissing(ch.target)
	}
	p.RichText(r.X+16, r.Y+10, r.W-90, head)
	if target != nil {
		open := ui.Span{Text: locales.BlocksEmbedOpen(), Size: 12, Color: cl.Muted}
		w, _ := p.MeasureText(0, open)
		p.RichText(r.X+r.W-w-14, r.Y+12, w+2, open)
	}
	if target == nil {
		return
	}
	y := r.Y + 10 + size*1.5 + 6
	bottom := r.Y + r.H - 8
	p.Clip(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, rad(11), func() {
		for _, ln := range a.embedLines(target) {
			ln = strings.TrimSpace(ln)
			bullet := ""
			switch {
			case strings.HasPrefix(ln, "- [ ]"):
				bullet, ln = "☐ ", strings.TrimSpace(ln[5:])
			case strings.HasPrefix(ln, "- [x]"), strings.HasPrefix(ln, "- [X]"):
				bullet, ln = "☑ ", strings.TrimSpace(ln[5:])
			case strings.HasPrefix(ln, "- "), strings.HasPrefix(ln, "* "):
				bullet, ln = "• ", strings.TrimSpace(ln[2:])
			}
			weight := 400
			if strings.HasPrefix(ln, "#") {
				ln, weight = strings.TrimSpace(strings.TrimLeft(ln, "#")), 650
			}
			spans := a.cellSpans(ln, size, weight, th, cl.Accent)
			if bullet != "" {
				spans = append([]ui.Span{{Text: bullet, Size: size, Color: cl.Muted}}, spans...)
			}
			_, h := p.MeasureText(r.W-36, spans...)
			if y+h > bottom+4 {
				break
			}
			p.RichText(r.X+16, y, r.W-36, spans...)
			y += h + 3
		}
		// the foot fades into the card
		fade := ui.Rect{X: r.X + 1, Y: r.Y + r.H - 40, W: r.W - 2, H: 39}
		bg := map[bool]ui.Color{true: cl.Fill, false: cl.Well}[hover]
		p.FillGradient(fade, ui.LinearGradient{Angle: 180, Oklab: true, Stops: []ui.GradientStop{{Color: bg.Alpha(0), At: 0}, {Color: bg, At: 1}}}, 0)
	})
}

// insertEmbed takes the "/…" away and puts the card of a page in a block of
// its own.
func (a *app) insertEmbed(start int, title string) {
	doc := a.pane().doc
	s := &a.slash
	_, caret := doc.Selection()
	doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.Select(start, start)
	})
	s.open = false
	a.putBlock(a.embedAttachment(&chip{kind: "embed", target: title}))
}
