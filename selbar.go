package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
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
}

var (
	iconBold   = icon(`<path d="M8 5h5a3.2 3.2 0 0 1 0 6.4H8zM8 11.4h5.8a3.3 3.3 0 0 1 0 6.6H8z"/>`)
	iconItalic = icon(`<path d="M10 5h7M7 19h7M13.5 5 10.5 19"/>`)
	iconStrike = icon(`<path d="M5 12h14M16.5 8a4 4 0 0 0-4-2.5c-2.3 0-4 1.2-4 3 0 1.3.9 2.2 3 2.8M7.5 16a4.5 4.5 0 0 0 4.5 2.5c2.5 0 4.3-1.2 4.3-3"/>`)
	iconLink   = icon(`<path d="M10 14a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1M14 10a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1"/>`)
)

// selectionBar draws the bar over the selection, when there is one to show.
func (a *app) selectionBar(c *ui.Context) {
	b := &a.sel
	if a.doc == nil || a.noPage() || a.page == nil || a.slash.open || a.ctx.open || a.grip.menu || a.dateEdit.open || a.modal() {
		b.link = false
		return
	}
	anchor, caret := a.doc.Selection()
	s, e := min(anchor, caret), max(anchor, caret)
	if b.link && [2]int{s, e} != b.linkSel {
		b.link = false
	}
	if s == e || !(a.focused || b.link) || (a.edPressed && !b.link) {
		b.link = false
		return
	}
	t := a.doc.Text()
	if t.ParagraphStyleAt(s).CodeBlock || strings.TrimSpace(t.Slice(s, e).String()) == "" {
		return
	}
	// where: over the start of the selection, under it when there is no room above
	cb, ok := a.locateRune(s, a.editorR.W)
	if !ok {
		return
	}
	x, y := a.editorR.X+cb.r.X, a.editorR.Y+cb.r.Y
	tth := ryugo.From(c)
	cl := tth.Colors
	f := a.doc.Formatting()
	w := float32(5*selBtn + 2*selPad + 4*2)
	ww, _ := c.Size()
	if b.link { // the field is as wide as the room: never past the window
		w = min(280, ww-16)
	}
	h := float32(selBtn + 2*selPad)
	left := max(8, min(x-8, ww-w-8))
	top := y - h - 10
	if top < a.scrollR.Y+a.barInset+4 {
		top = y + cb.r.H + 10
	}
	ui.Overlay(c, func() {
		bar := ui.Row(c.Key("selbar")).Absolute().Left(left).Top(top).Width(w).Height(h).Padding(selPad).Gap(2).AlignItems(ui.Center).Clip().
			Radius(tth.R(ryugo.RadiusControl)).Background(cl.Card).Border(1, cl.Hairline).Label(T("Biçim çubuğu")).KeepFocus()
		tth.Shadows.ShadowPop.Apply(bar)
		bar.Children(func() {
			if b.link {
				in := a.fieldMenu(ryugo.Input(c.Key("selbar-url"), &b.url, ryugo.InputOptions{Label: T("Bağlantı adresi"), Placeholder: T("Adres yapıştır, ↵"), PrefixIcon: iconLink, Size: ryugo.Sm}), &b.url, fieldOpts{oneLine: true}).
					Grow(1).MinWidth(0).AutoFocus()
				switch {
				case in.Shortcut(0, ui.KeyEscape):
					b.link = false
					a.refocus = true
				case in.Submitted():
					a.doc.SetSelection(b.linkSel[0], b.linkSel[1])
					a.doc.SetLink(strings.TrimSpace(b.url))
					b.link = false
					a.refocus = true
				}
				return
			}
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
			btn("sb-bold", iconBold, "Kalın (⌘B)", f.Bold == richtext.FormatOn, a.doc.ToggleBold)
			btn("sb-italic", iconItalic, "İtalik (⌘I)", f.Italic == richtext.FormatOn, a.doc.ToggleItalic)
			btn("sb-strike", iconStrike, "Üstü çizili", f.Strikethrough == richtext.FormatOn, a.doc.ToggleStrikethrough)
			btn("sb-code", kindIcon["code"], "Kod", f.Code == richtext.FormatOn, a.doc.ToggleCode)
			btn("sb-link", iconLink, "Bağlantı (⌘K)", f.Link != "", func() {
				if f.Link != "" {
					a.doc.SetLink("")
					return
				}
				b.link, b.url, b.linkSel = true, "", [2]int{s, e}
			})
		})
	})
}
