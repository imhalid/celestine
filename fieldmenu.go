package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// The app's own menu on every text: MyGo gives a text input, a selectable text and the editor a
// menu of the system's, which takes the right click before the app sees it. Each is given a
// ContextMenu whose build opens the app's menu instead (ctxmenu.go) and adds nothing to the
// system's, which then does not show. A field's menu cuts, copies, pastes and selects its text, and
// has what the place it is in offers after a line.

// fieldOpts says what a field's menu has besides its text's lines.
type fieldOpts struct {
	oneLine bool              // a paste is put on one line
	secret  bool              // a password: nothing of it is copied
	changed func()            // after the menu changed the text
	more    func() []menuItem // after a line
	editing func() bool       // when set, the text's lines only while it says the field is being edited; else only more
}

// fieldSel is a selection the menu left in a field, to be set on it as it is built next.
type fieldSel struct{ s, e int }

// fieldMenu gives text input el, which edits value, the app's menu; it returns el.
func (a *app) fieldMenu(el ui.Element, value *string, o fieldOpts) ui.Element {
	if s, ok := a.fieldSels[value]; ok {
		delete(a.fieldSels, value)
		el.Focus()
		el.SetTextSelection(s.s, s.e)
	}
	el.ContextMenu(func(*ui.Menu) {
		px, py, _ := el.PointerPosition()
		b := el.LayoutBounds()
		items := a.fieldItems(el, value, o)
		if o.editing != nil && !o.editing() && o.more != nil { // not el.Focused(): the right click has given it the focus
			items = o.more()
		}
		if len(items) > 0 {
			a.showContext(b.X+px, b.Y+py, items)
		}
	})
	return el
}

// fieldItems are the lines of a field's menu.
func (a *app) fieldItems(el ui.Element, value *string, o fieldOpts) []menuItem {
	s, e := el.TextSelection()
	r := []rune(*value)
	s, e = max(0, min(s, e, len(r))), min(max(s, e), len(r))
	set := func(t string, at int) {
		*value = t
		if a.fieldSels == nil {
			a.fieldSels = map[*string]fieldSel{}
		}
		a.fieldSels[value] = fieldSel{at, at}
		if o.changed != nil {
			o.changed()
		}
	}
	var out []menuItem
	if s < e && !o.secret {
		sel := string(r[s:e])
		out = append(out,
			menuItem{title: "common_cut", hint: "⌘X", glyph: iconCut, run: func() {
				a.copyText(sel)
				set(string(r[:s])+string(r[e:]), s)
			}},
			menuItem{title: "common_copy", hint: "⌘C", glyph: iconCopy, run: func() {
				a.copyText(sel)
				if a.fieldSels == nil {
					a.fieldSels = map[*string]fieldSel{}
				}
				a.fieldSels[value] = fieldSel{s, e} // the field keeps the focus and its selection, as after the others
			}})
	}
	out = append(out,
		menuItem{title: "common_paste", hint: "⌘V", glyph: iconPaste, run: func() {
			clip := a.readClip()
			if o.oneLine {
				clip = strings.Join(strings.Fields(clip), " ")
			}
			in := []rune(clip)
			set(string(r[:s])+string(in)+string(r[e:]), s+len(in))
		}},
		menuItem{title: "common_selectAllText", hint: "⌘A", glyph: iconSelectAll, run: func() {
			if a.fieldSels == nil {
				a.fieldSels = map[*string]fieldSel{}
			}
			a.fieldSels[value] = fieldSel{0, len(r)}
		}})
	return withMore(out, o.more)
}

// textMenu gives a text that can be selected, not changed, the app's menu: its text to copy, and
// what the place offers.
func (a *app) textMenu(el ui.Element, text func() string, more func() []menuItem) ui.Element {
	el.ContextMenu(func(*ui.Menu) {
		px, py, _ := el.PointerPosition()
		b := el.LayoutBounds()
		items := withMore([]menuItem{{title: "common_copy", hint: "⌘C", glyph: iconCopy, run: func() { a.copyText(text()) }}}, more)
		a.showContext(b.X+px, b.Y+py, items)
	})
	return el
}

// withMore puts what the place offers after the text's lines, a line between them.
func withMore(out []menuItem, more func() []menuItem) []menuItem {
	if more == nil {
		return out
	}
	for i, it := range more() {
		if i == 0 {
			it.group = "~"
		}
		out = append(out, it)
	}
	return out
}

// editorMenu is the page editor's menu (richtext.EditorOptions.Menu): the app's, where the pointer
// was last seen over the text.
func (a *app) editorMenu(*ui.Menu) {
	p := a.edPtr
	a.showContext(a.editorR.X+p.X, a.editorR.Y+p.Y, a.editorItems(p.Y))
}

// editorItems are the lines of the editor's menu, the pointer at y in it: the selection's text and
// its formats, then the block under the pointer.
func (a *app) editorItems(y float32) []menuItem {
	doc := a.doc
	sa, sc := doc.Selection()
	s, e := min(sa, sc), max(sa, sc)
	var out []menuItem
	if s < e {
		md := strings.TrimSuffix(bodyMarkdown(doc.Text().Slice(s, e)), "\n")
		out = append(out,
			menuItem{title: "common_cut", hint: "⌘X", glyph: iconCut, run: func() {
				a.copyText(md)
				doc.Edit(func(ed *richtext.Edit) { ed.Replace(s, e, richtext.Text{}); ed.Select(s, s) })
				a.refocus = true
			}},
			menuItem{title: "common_copy", hint: "⌘C", glyph: iconCopy, run: func() { a.copyText(md); a.refocus = true }},
			a.into(locales.PageCopyAs(), iconCopy, func() []menuItem {
				return []menuItem{
					{title: "Markdown", hint: "⇧⌘C", glyph: iconCopy, run: func() { a.copyAs("markdown") }},
					{title: locales.PagePlainText(), glyph: iconCopy, run: func() { a.copyAs("plain") }},
					{title: "HTML", glyph: iconCopy, run: func() { a.copyAs("html") }},
				}
			}))
	}
	out = append(out, a.into(locales.PagePasteAs(), iconPaste, func() []menuItem {
		return []menuItem{
			{title: "Markdown", hint: "⌥⌘V", glyph: iconPaste, run: func() { a.pasteAs("markdown") }},
			{title: locales.PagePlainText(), hint: "⇧⌘V", glyph: iconPaste, run: func() { a.pasteAs("plain") }},
		}
	}))
	out = append(out,
		menuItem{title: "common_paste", hint: "⌘V", glyph: iconPaste, run: func() {
			if clip := a.readClip(); clip != "" {
				if strings.Contains(clip, "\n") {
					doc.Insert(importMarkdown(clip))
				} else {
					doc.InsertText(clip)
				}
			}
			a.refocus = true
		}},
		menuItem{title: "common_selectAllText", hint: "⌘A", glyph: iconSelectAll, run: func() {
			doc.SetSelection(0, doc.Text().Len())
			a.refocus = true
		}})
	if s < e {
		for i, f := range []struct {
			name, keys string
			do         func()
		}{{"common_bold", "⌘B", doc.ToggleBold}, {"common_italic", "⌘I", doc.ToggleItalic}, {"common_underline", "⌘U", doc.ToggleUnderline},
			{"common_strike", "", doc.ToggleStrikethrough}, {"code", "⌘E", doc.ToggleCode}} {
			f := f
			it := menuItem{title: f.name, hint: f.keys, glyph: iconFormat, run: func() { f.do(); a.refocus = true }}
			if i == 0 {
				it.group = "~"
			}
			out = append(out, it)
		}
	}
	if box, ok := a.paraAtY(y); ok { // the block under the pointer, as its grip's menu has it
		a.grip.para = box.Start
		first := true
		for _, it := range a.blockItems() { // Dönüştür… is a submenu of its kinds
			if first {
				it.group, first = "~", false
			}
			out = append(out, it)
		}
	}
	return out
}
