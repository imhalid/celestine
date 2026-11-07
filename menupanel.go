package main

import (
	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
	"strconv"
)

// The menu panel the / and @ menu and the block's menu share: a header, a
// divider, an optional field, and a list of rows - grouped, with icons, a
// second line, a hint on the right, a ›  on those that go into a menu -
// that scrolls when there is no room.

// placed is where a menu goes: beside its anchor, inside the window.
type placed struct {
	left, top, inner float32
	tops, ends       []float32
}

// placeMenu puts a menu of items below its anchor (a span from atop to
// abot at ax), above it when there is more room there; at most the
// window's height, and inside its width. extra is what is under the rows,
// head the header and what is under it.
func placeMenu(c *ui.Context, ax, atop, abot float32, items []menuItem, extra, head float32) placed {
	ww, wh := c.Size()
	tops, ends, total := menuRows(items, extra)
	chrome := float32(2*menuPad+2) + head // the padding, the border and the header
	below := wh - (abot + menuGap) - 8 - chrome
	above := atop - menuGap - 8 - chrome
	inner, top := min(total, below), abot+menuGap
	if total > below && above > below {
		inner = min(total, above)
		top = atop - menuGap - inner - chrome
	}
	return placed{left: max(8, min(ax, ww-menuW-8)), top: top, inner: max(inner, menuRow), tops: tops, ends: ends}
}

// keepInSight scrolls the list to the choice, when it has moved.
func keepInSight(sel int, seen *int, scroll *ui.ScrollState, pl placed) {
	if sel != *seen && sel >= 0 && sel < len(pl.tops) {
		if pl.tops[sel] < scroll.Y {
			scroll.Y = pl.tops[sel]
		} else if pl.ends[sel] > scroll.Y+pl.inner {
			scroll.Y = pl.ends[sel] - pl.inner
		}
	}
	*seen = sel
}

// chosenFill is the fill of the row chosen: the accent as a button has it,
// a gentle turn from top to bottom, softer than the flat accent.
func chosenFill(t *ryugo.Tokens) ui.LinearGradient {
	cl := t.Colors
	return ui.LinearGradient{Angle: 180, Oklab: true,
		From: ryugo.Mix(cl.ColorAccent500, cl.ColorAccent600, 0.7), To: ryugo.Mix(cl.ColorAccent600, cl.ColorAccent700, 0.5)}
}

// menuPanel is a menu to draw.
type menuPanel struct {
	key, label string
	pl         placed
	head       func() // the header's contents; nil for no header
	field      func() // a field under the header, or nil
	items      []menuItem
	sel        int
	selPtr     *int // the caller's choice, which a hovered row takes
	hoverSeen  *int // the row last hovered, so a keyboard move is not undone by a pointer at rest
	scroll     *ui.ScrollState
	empty      string // what is said when there is nothing to list
	under      func() // under the rows: a calendar
	choose     func(menuItem)
}

// drawMenu draws the panel, in an overlay, and returns it.
func (a *app) drawMenu(c *ui.Context, t *ryugo.Tokens, m *menuPanel) ui.Element {
	cl := t.Colors
	panel := ui.Column(c.Key(m.key)).Absolute().Left(m.pl.left).Top(m.pl.top).Width(menuW).Padding(menuPad).
		Radius(rad(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label(m.label)
	t.Shadows.ShadowPop.Apply(panel)
	panel.Children(func() {
		if m.head != nil { // a menu may have none
			ui.Row(c.Key(m.key+"-head")).Height(menuTop).Padding(0, 6).AlignItems(ui.Center).Gap(8).Children(m.head)
			ui.Box(c.Key(m.key+"-d1")).Height(1).Background(cl.Hairline).Margin(0, -menuPad)
			ui.Box(c.Key(m.key + "-s1")).Height(menuAfter)
		}
		if m.field != nil {
			ui.Row(c.Key(m.key+"-field")).Height(menuIn).Padding(0, 6).Gap(8).AlignItems(ui.Center).Children(m.field)
			ui.Box(c.Key(m.key+"-d2")).Height(1).Background(cl.Hairline).Margin(0, -menuPad)
			ui.Box(c.Key(m.key + "-s2")).Height(menuAfter)
		}
		a.scrolled(c, "menupanel:91", m.scroll, ui.Scroll(c).Height(m.pl.inner), func() {
			ui.Column(c).Gap(1).Children(func() {
				if len(m.items) == 0 {
					ui.Column(c.Key("empty")).Height(menuEmpty).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Icon(c, iconNone).Width(24).Height(24).TextColor(cl.Muted.Alpha(0.6))
						ui.Text(c, m.empty).FontSize(12).TextColor(cl.Muted)
					})
				}
				if len(m.items) == 1 && m.items[0].back { // nothing but the way back: said under it
					defer ui.Column(c.Key("empty2")).Height(menuEmpty).Justify(ui.Center).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Icon(c, iconNone).Width(24).Height(24).TextColor(cl.Muted.Alpha(0.6))
						ui.Text(c, m.empty).FontSize(12).TextColor(cl.Muted)
					})
				}
				group := ""
				for i, it := range m.items {
					if it.group != group {
						group = it.group
						switch group {
						case "":
						case "~":
							ui.Column(c.Key("sep"+strconv.Itoa(i))).Height(menuSep).Padding(0, 10).Justify(ui.Center).Children(func() {
								ui.Box(c).Height(1).Background(cl.Hairline)
							})
						default:
							ui.Text(c.Key("g:"+group), group).FontSize(10.5).FontWeight(650).TextColor(cl.Muted).
								Height(menuHead).Padding(7, 8, 0, 8)
						}
					}
					row := ui.Row(c.Key(i)).Height(menuRowH(it)).Padding(0, 8).Gap(8).Radius(rad(ryugo.RadiusControl - 4)).
						AlignItems(ui.Center).Label(it.title).Role(ui.RoleButton)
					text, muted := cl.Text, cl.Muted
					if it.danger {
						text, muted = cl.Up, cl.Up.Alpha(0.8)
					}
					// one row is lit: the one the keys are on, or the one the pointer
					// came to last
					if row.Hovered() && m.hoverSeen != nil && *m.hoverSeen != i {
						*m.hoverSeen = i
						if m.selPtr != nil {
							*m.selPtr = i
						}
						m.sel = i
						c.Invalidate()
					}
					if i == m.sel {
						row.LinearGradient(chosenFill(t))
						text, muted = cl.OnAccent, cl.OnAccent.Alpha(0.75)
					}
					row.Children(func() {
						if it.dot != nil {
							ui.Box(c).Width(13).Height(13).Shrink(0).Center().Children(func() {
								ui.Box(c).Size(8, 8).Radius(rad(3)).Background(*it.dot).Border(1, cl.OnAccent.Alpha(map[bool]float32{true: 0.7, false: 0}[i == m.sel]))
							})
						} else if it.glyph != nil {
							ui.Icon(c, it.glyph).Width(13).Height(13).TextColor(muted)
						}
						ui.Column(c).Grow(1).Shrink(1).Children(func() {
							ui.Text(c, it.title).FontSize(12.5).TextColor(text).MaxLines(1)
							if it.sub != "" {
								ui.Text(c, it.sub).FontSize(10.5).TextColor(muted).MaxLines(1)
							}
						})
						if it.hint != "" {
							ui.Text(c, it.hint).FontSize(11.5).TextColor(muted).MaxLines(1)
						}
						if it.nav && !it.back {
							ui.Icon(c, iconChevron).Width(12).Height(12).TextColor(muted)
						}
					})
					if row.Clicked() {
						m.choose(it)
					}
				}
				if m.under != nil {
					m.under()
				}
			})
		})
	})
	return panel
}
