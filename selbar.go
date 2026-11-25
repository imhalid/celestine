package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// The bar over a selection: bold, italic, strikethrough, code, a link. It
// comes up under the mouse's release over text selected, as Notion's and
// Thymer's do, in the design of the app's menus; what is on shows. It keeps
// the editor's focus, so the selection stays. A link asks for its address in
// the bar itself.

const (
	selBtn = 24 // a button of the bar
	selPad = 4  // its padding
)

type selBar struct {
	link    bool   // asking for a link's address
	url     string // what was typed
	linkSel [2]int // what the link is for
	// what was typed as the last frame had it, and since when: the page is asked for once the typing rests (#495)
	urlWas string
	urlAt  time.Time
	// the link just set, and when, for its text's flourish (linkfx.go, #509)
	wonSel [2]int
	wonAt  time.Time
}

// linkTypeRest is how long the address typed rests before its page is asked for.
const linkTypeRest = 350 * time.Millisecond

var (
	iconBold   = icon(`<path d="M8 5h5a3.2 3.2 0 0 1 0 6.4H8zM8 11.4h5.8a3.3 3.3 0 0 1 0 6.6H8z"/>`)
	iconItalic = icon(`<path d="M10 5h7M7 19h7M13.5 5 10.5 19"/>`)
	iconStrike = icon(`<path d="M5 12h14M16.5 8a4 4 0 0 0-4-2.5c-2.3 0-4 1.2-4 3 0 1.3.9 2.2 3 2.8M7.5 16a4.5 4.5 0 0 0 4.5 2.5c2.5 0 4.3-1.2 4.3-3"/>`)
	iconLink   = icon(`<path d="M10 14a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1M14 10a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1"/>`)
)

// selectionBar draws the bar over the selection, when there is one to show.
func (a *app) selectionBar(c *ui.Context) {
	b := &a.pane().sel
	if a.pane().doc == nil || a.noPage() || a.pane().page == nil || a.slash.open || a.ctx.open || a.pane().grip.menu || a.dateEdit.open || a.modal() {
		b.link = false
		return
	}
	anchor, caret := a.pane().doc.Selection()
	s, e := min(anchor, caret), max(anchor, caret)
	if b.link && [2]int{s, e} != b.linkSel {
		b.link = false
	}
	if s == e || !(a.pane().focused || b.link) || (a.pane().edPressed && !b.link) {
		b.link = false
		return
	}
	t := a.pane().doc.Text()
	if t.ParagraphStyleAt(s).CodeBlock || strings.TrimSpace(t.Slice(s, e).String()) == "" {
		return
	}
	// where: over the start of the selection, under it when there is no room above
	cb, ok := a.locateRune(a.pane(), s, a.pane().editorR.W)
	if !ok {
		return
	}
	x, y := a.pane().editorR.X+cb.r.X, a.pane().editorR.Y+cb.r.Y
	tth := ryugo.From(c)
	cl := tth.Colors
	f := a.pane().doc.Formatting()
	w := float32(6*selBtn + 2*selPad + 5*2)
	ww, _ := c.Size()
	h := float32(selBtn + 2*selPad)
	full := h // the bar with the preview of the link typed, under the field
	var page *linkInfo
	address := strings.TrimSpace(b.url)
	if b.link { // the field is as wide as the room: never past the window
		w = min(340, ww-16)
		h = 38
		full = h
		if b.url != b.urlWas {
			b.urlWas, b.urlAt = b.url, c.Now()
		}
		if rest := linkTypeRest - c.Now().Sub(b.urlAt); rest > 0 {
			c.After(rest)
		} else if host := linkHost(address); strings.Contains(host, ".") {
			page = a.linkPage(address, true)
		}
		if page != nil {
			full += 1 + linkPreviewH(page, w)
		}
	}
	left := max(8, min(x-8, ww-w-8))
	top := y - full - 10
	if top < a.pane().scrollR.Y+a.pane().barInset+4 {
		top = y + cb.r.H + 10
	}
	ui.Overlay(c, func() {
		if b.link {
			a.linkField(c, b, page, address, left, top, w, h)
			return
		}
		bar := ui.Row(c.Key("selbar")).Absolute().Left(left).Top(top).Width(w).Height(h).Padding(selPad).Gap(2).AlignItems(ui.Center).Clip().
			Radius(tth.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label(locales.SelectionFormatBar()).KeepFocus()
		tth.Shadows.ShadowPop.Apply(bar)
		bar.Children(func() {
			btn := func(key string, glyph *ui.SVG, label string, on bool, fn func()) {
				e := ui.Box(c.Key(key)).Size(selBtn, selBtn).Radius(rad(6)).AlignItems(ui.Center).Justify(ui.Center).Label(label).Role(ui.RoleButton).
					Cursor(ui.CursorPointer).Transition(ui.ElementTransition{Duration: 100 * 1e6, Colors: true})
				ryugo.Tooltip(c, e, label)
				switch {
				case on:
					e.Background(cl.Accent.Alpha(0.18))
				case e.Hovered():
					e.Background(cl.Fill)
				}
				col := cl.Text2
				if on {
					col = cl.Accent
				}
				e.Children(func() { ui.Icon(c, glyph).Width(15).Height(15).TextColor(col) })
				if e.Clicked() {
					fn()
				}
			}
			btn("sb-bold", iconBold, locales.SelectionBold(), f.Bold == richtext.FormatOn, a.pane().doc.ToggleBold)
			btn("sb-italic", iconItalic, locales.SelectionItalic(), f.Italic == richtext.FormatOn, a.pane().doc.ToggleItalic)
			btn("sb-strike", iconStrike, locales.SelectionStrike(), f.Strikethrough == richtext.FormatOn, a.pane().doc.ToggleStrikethrough)
			btn("sb-code", kindIcon["code"], "code", f.Code == richtext.FormatOn, a.pane().doc.ToggleCode)
			btn("sb-link", iconLink, locales.SelectionLink(), f.Link != "", func() {
				// a link there is edited: its address in the field, which emptied takes it away (#509)
				b.link, b.url, b.linkSel = true, f.Link, [2]int{s, e}
			})
			btn("sb-extract", iconArrowRight, locales.ExtractToANewPage(), false, a.extractToPage)
		})
	})
}

// linkField is the bar asking for a link's address (#495): the field in the bar's own look, a key cap that says ↵
// sets it, and under it, once the typing rests on a web address, its page's picture, title and icon as they come,
// the picture as large as the hover's card has it (#509).
func (a *app) linkField(c *ui.Context, b *selBar, page *linkInfo, address string, left, top, w, h float32) {
	tth := ryugo.From(c)
	cl := tth.Colors
	box := ui.Column(c.Key("selbar")).Absolute().Left(left).Top(top).Width(w).Clip().
		Radius(tth.R(ryugo.RadiusControl)+2).Background(cl.Card).Border(1, cl.Hairline).Label(locales.SelectionFormatBar()).KeepFocus().
		Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, Size: true})
	tth.Shadows.ShadowPop.Apply(box)
	box.Children(func() {
		ui.Row(c.Key("selbar-link")).Height(h).Padding(0, 8, 0, 12).Gap(9).AlignItems(ui.Center).Children(func() {
			ui.Box(c.Key("selbar-link-icon")).Width(15).Height(15).Draw(func(p *ui.Painter, r ui.Rect) {
				if webLink(address) && page != nil {
					a.drawLinkIcon(p, r, address, cl.Muted)
					return
				}
				p.Icon(iconLink, r, cl.Muted)
			})
			field := ui.TextInputBase(c.Key("selbar-url"), &b.url).Grow(1).MinWidth(0).FontSize(13.5).TextColor(cl.Text).
				Placeholder(locales.SelectionPasteAddress()).PlaceholderColor(cl.Muted).FocusRing(false).Label(locales.SelectionLinkAddress())
			in := a.fieldMenu(field, &b.url, fieldOpts{oneLine: true}).AutoFocus()
			if address != "" { // the key that sets it
				ui.Box(c.Key("selbar-enter")).Height(20).Padding(0, 6).Radius(rad(5)).Background(cl.Fill).Border(1, cl.Hairline).
					AlignItems(ui.Center).Justify(ui.Center).Transition(rowEnter).Children(func() {
					ui.Text(c, "↵").FontSize(12).TextColor(cl.Text2)
				})
			}
			switch {
			case in.Shortcut(0, ui.KeyEscape):
				b.link = false
				a.pane().refocus = true
			case in.Submitted():
				a.pane().doc.SetSelection(b.linkSel[0], b.linkSel[1])
				a.pane().doc.SetLink(address)
				if address != "" {
					b.wonSel, b.wonAt = b.linkSel, c.Now()
				}
				b.link = false
				a.pane().refocus = true
			}
		})
		if page != nil {
			ui.Box(c.Key("selbar-link-line")).Height(1).Background(cl.Hairline)
			ui.Column(c.Key("selbar-link-page")).Transition(rowEnter).Children(func() {
				a.linkPreview(c, address, page, w)
			})
		}
	})
}
