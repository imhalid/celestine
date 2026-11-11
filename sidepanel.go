package main

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The side panel, floating at the window's right while a page is open: its outline,
// the pages that link to it, and what there is to know of it. A small card over the page, not a
// column of the layout; it comes in from the right and stays as it was left.

var (
	iconPanel   = icon(`<rect x="3.5" y="4.5" width="17" height="15" rx="2.5"/><path d="M14.5 4.5v15"/>`)
	iconOutline = icon(`<path d="M5 6.5h14M8 11h11M11 15.5h8M11 20h5"/>`)
)

func (a *app) togglePanel() {
	a.settings.PanelOpen = !a.settings.PanelOpen
	a.saveSettings()
}

// heading is a line of the outline: where it is, how deep, what it says.
type heading struct {
	at, level int
	text      string
}

// outline is the headings of the page's text, in order.
func (a *app) outline() []heading {
	if a.doc == nil {
		return nil
	}
	if v := a.doc.Version(); v != a.outlineVer || a.outlineDoc != a.doc {
		a.outlineOf, a.outlineVer, a.outlineDoc = nil, v, a.doc
		t := a.doc.Text()
		pos := 0
		for _, ln := range strings.Split(t.String(), "\n") {
			n := utf8.RuneCountInString(ln)
			if st := t.ParagraphStyleAt(min(pos, max(t.Len()-1, 0))); st.Heading > 0 && strings.TrimSpace(ln) != "" {
				a.outlineOf = append(a.outlineOf, heading{pos, st.Heading, strings.TrimSpace(strings.ReplaceAll(ln, string(richtext.ObjectReplacement), ""))})
			}
			pos += n + 1
		}
	}
	return a.outlineOf
}

// gotoPos puts the caret at at and brings it into view.
func (a *app) gotoPos(at int) {
	a.doc.SetSelection(at, at)
	a.revealWant, a.refocus = true, true
}

// panelFloat is the panel's coming and going: in from the right, out the same way.
var panelFloat = ui.ElementTransition{
	Animation: &ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond}},
	Enter:     &ui.Motion{Opacity: 0, X: 18},
	Exit:      &ui.Motion{Opacity: 0, X: 18},
}

// sidePanel is the panel: a small card that floats over the page at its right, under the title bar,
// as tall as what it holds up to the window's height. It takes no room from the text.
func (a *app) sidePanel(c *ui.Context) {
	if !(a.settings.PanelOpen && a.page != nil && !a.noPage()) {
		return
	}
	t := ryugo.From(c)
	cl := t.Colors
	p := a.page
	top := a.contentTop(c) + 10
	maxH := float32(560)
	if a.winRect.H > 0 {
		maxH = max(220, a.winRect.H-top-44) // the status bar stays clear
	}
	full := a.panelWidth()
	card := ui.Column(c.Key("side-panel")).Absolute().Top(top).Right(14).Width(full).Radius(rad(ryugo.RadiusControl+4)).
		Background(cl.Card).Border(1, cl.Hairline).Clip().Label(locales.CommonSidePanel())
	t.Shadows.ShadowPop.Apply(card)
	card.Transition(panelFloat)
	card.Children(func() {
		defer a.edgeGrip(c, "panel-grip", false, full, &a.settings.PanelWidth, &a.edge.panel) // over the content
		ui.Row(c).FillWidth().Height(34).Padding(0, 8, 0, 14).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.CommonThisPage()).FontSize(11).FontWeight(650).TextColor(cl.Muted).Grow(1)
			if a.iconButton(c, iconClose, "Paneli kapat") {
				a.togglePanel()
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(cl.Hairline)
		a.scrolled(c, "panel-scroll", nil, ui.Scroll(c.Key("panel-scroll")).FillWidth().MaxHeight(maxH-36), func() {
			ui.Column(c).FillWidth().Padding(10, 14, 18, 14).Gap(18).Children(func() {
				a.panelOutline(c, cl)
				a.panelTags(c, cl)
				a.panelLinks(c, cl, p)
				a.panelInfo(c, cl, p)
			})
		})
	})
}

// panelTitle is a section's heading.
func panelTitle(c *ui.Context, cl ryugo.Colors, s string) {
	ui.Text(c, s).FontSize(11).FontWeight(650).TextColor(cl.Muted).Padding(0, 6, 2, 6)
}

func (a *app) panelOutline(c *ui.Context, cl ryugo.Colors) {
	ui.Column(c.Key("panel-outline")).FillWidth().Gap(1).Children(func() {
		panelTitle(c, cl, "İÇİNDEKİLER")
		hs := a.outline()
		if len(hs) == 0 {
			ui.Text(c, locales.PanesThePageHasNo()).FontSize(12.5).TextColor(cl.Muted).Padding(2, 6).Wrap()
			return
		}
		least := 6
		for _, h := range hs {
			least = min(least, h.level)
		}
		for i, h := range hs {
			h := h
			row := ui.Row(c.Key("ol:"+strconv.Itoa(i))).FillWidth().Height(28).Padding(0, 6, 0, 6+float32(h.level-least)*12).AlignItems(ui.Center).
				Radius(7).Label(locales.PanesHeadingLabel(h.text)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Background(cl.Fill)
			}
			row.Children(func() {
				ui.Text(c, h.text).FontSize(13).FontWeight(map[bool]int{true: 620, false: 450}[h.level == least]).
					TextColor(map[bool]ui.Color{true: cl.Text, false: cl.Text2}[h.level == least]).MaxLines(1)
			})
			if row.Clicked() {
				a.gotoPos(h.at)
			}
		}
	})
}

func (a *app) panelLinks(c *ui.Context, cl ryugo.Colors, p *Page) {
	links := a.linksTo(p, a.clock(), false)
	ui.Column(c.Key("panel-links")).FillWidth().Gap(1).Children(func() {
		panelTitle(c, cl, "BAĞLANANLAR")
		if len(links) == 0 {
			ui.Text(c, locales.PanesNothingLinksThisPage()).FontSize(12.5).TextColor(cl.Muted).Padding(2, 6).Wrap()
			return
		}
		for _, q := range links {
			q := q
			row := ui.Column(c.Key("pl:"+q.Dir)).FillWidth().Padding(5, 6).Radius(rad(7)).Label(locales.CommonLinkLabel(q.Title)).Role(ui.RoleButton).Cursor(ui.CursorPointer)
			if row.Hovered() {
				row.Background(cl.Fill)
			}
			a.contextOn(row, func() []menuItem { return a.pageMenu(q, false) })
			row.Children(func() {
				ui.Text(c, q.Title).FontSize(13).TextColor(cl.Text).MaxLines(1)
				if segs := a.linkContext(q, p); len(segs) > 0 {
					ui.RichText(c, contextSpans(segs, 11.5, cl.Muted, cl.Text, cl.Accent.Alpha(0.18))...).MaxLines(3)
				} else {
					ui.Text(c, q.Coll.Name).FontSize(11.5).TextColor(cl.Muted).MaxLines(1)
				}
			})
			if row.Clicked() {
				a.open(q)
			}
		}
	})
}

func (a *app) panelInfo(c *ui.Context, cl ryugo.Colors, p *Page) {
	words, chars := a.docStats()
	rows := [][2]string{
		{"Koleksiyon", p.Coll.Name},
		{"Oluşturuldu", shortDate(p.Created)},
		{"Değiştirildi", ago(p.Modified, a.clock())},
		{"Kelime", strconv.Itoa(words)},
		{"Harf", strconv.Itoa(chars)},
		{"Okuma süresi", readingTime(words)},
	}
	if n := len(a.linksTo(p, a.clock(), false)); n > 0 {
		rows = append(rows, [2]string{"Geri bağlantı", strconv.Itoa(n)})
	}
	if n := len(p.Children); n > 0 {
		rows = append(rows, [2]string{"Alt sayfa", strconv.Itoa(n)})
	}
	if p.Parent != nil {
		rows = append(rows, [2]string{"Üst sayfa", p.Parent.Title})
	}
	ui.Column(c.Key("panel-info")).FillWidth().Gap(1).Children(func() {
		panelTitle(c, cl, "BİLGİ")
		for _, r := range rows {
			if r[1] == "" {
				continue
			}
			ui.Row(c.Key("pi:"+r[0])).FillWidth().Padding(4, 6).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, r[0]).FontSize(12.5).TextColor(cl.Muted).Shrink(0)
				ui.Text(c, r[1]).FontSize(12.5).TextColor(cl.Text2).Grow(1).MaxLines(1).TextAlign(ui.End)
			})
		}
	})
}

// readingTime is how long the words take to read, at 200 a minute.
func readingTime(words int) string {
	if words == 0 {
		return ""
	}
	if m := (words + 199) / 200; m > 1 {
		return strconv.Itoa(m) + " dk"
	}
	return "1 dk'dan az"
}
