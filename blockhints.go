package main

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// The status bar's hints while a page is open are those of where the caret is (#270): the formats of a selection,
// what a heading, a list item, a block of code or an empty line can be made to do. The keys everyone knows (⌘P, ⌘K,
// ⌘F) and those of little use are left to the palette, which lists them all.

// cmdKey is the key of app command title as it is now, shown as the palette shows it: the user's own binding, or the
// command's (appcmds.go); "" when it has none.
func (a *app) cmdKey(title string) string {
	if k, ok := a.settings.Keys[title]; ok {
		return accelLabel(k)
	}
	for _, cmd := range appCommands() {
		if cmd.title == title {
			return accelLabel(cmd.accel)
		}
	}
	return ""
}

// keyPair is two keys that differ in their last sign, as one: ⇧⌘↑ and ⇧⌘↓ are ⇧⌘↑↓.
func keyPair(x, y string) string {
	if x == "" || y == "" {
		return x + y
	}
	xr, yr := []rune(x), []rune(y)
	if len(xr) == len(yr) && string(xr[:len(xr)-1]) == string(yr[:len(yr)-1]) {
		return x + string(yr[len(yr)-1])
	}
	return x + " " + y
}

// codeKey is the editor's key for code: ⌘E on macOS, where ⌘E elsewhere centers the line.
func codeKey() string {
	if runtime.GOOS == "darwin" {
		return "⌘E"
	}
	return "⇧⌘E"
}

// editorHints are the hints of the text, by the block the caret is in.
func (a *app) editorHints() []hint {
	var out []hint
	add := func(key, what string) {
		if key != "" {
			out = append(out, hint{key, what})
		}
	}
	if a.zoomed() {
		add(a.cmdKey("Uzaklaş"), "zoom out")
	}
	anchor, caret := a.doc.Selection()
	if anchor != caret { // a selection: its formats
		add("⌘B", "bold")
		add("⌘I", "italic")
		add(codeKey(), "code")
		add("⌘U", "underline")
		add("⇧⌘X", "strikethrough")
		return out
	}
	t := a.doc.Text()
	st := t.ParagraphStyleAt(caret)
	ps, pe := t.ParagraphRange(caret)
	empty := strings.TrimSpace(t.Slice(ps, pe).String()) == ""
	move := keyPair(a.cmdKey("Bloğu yukarı taşı"), a.cmdKey("Bloğu aşağı taşı"))
	kids, folded := blockHasKids(t, caret), foldOf(t, caret) != nil
	bs, be := blockRange(t, caret)
	zoomable := kids && !(bs == 0 && be >= t.Len()) // zoomIn does not zoom into the whole page
	foldHint := func() {
		switch {
		case folded:
			add(a.cmdKey("Katla / aç"), "unfold")
		case kids:
			add(a.cmdKey("Katla / aç"), "fold")
		}
	}
	switch {
	case st.Heading > 0:
		if zoomable {
			add(a.cmdKey("Bloğa yakınlaş"), "zoom into the section")
		}
		foldHint()
		add("⌥⌘1–6", "heading level")
		add("⌥⌘0", "plain text")
		add(move, "move")
	case st.List != richtext.NoList && empty && st.Indent > 0: // ↵ on an empty item takes it a level out
		add("↵", "outdent")
	case st.List != richtext.NoList && empty: // and at the first level ends the list
		add("↵", "end the list")
	case st.Quote && empty:
		add("↵", "end the quote")
	case st.List != richtext.NoList:
		add("Tab", "indent")
		add("⇧Tab", "outdent")
		if zoomable {
			add(a.cmdKey("Bloğa yakınlaş"), "zoom into the item")
		}
		foldHint()
		add(move, "move")
	case st.CodeBlock && empty && ps > 0 && t.ParagraphStyleAt(ps-1).CodeBlock:
		add("↵", "end the code")
	case st.CodeBlock:
		add("Tab", "indent")
		add("⇧Tab", "outdent")
		add(move, "move")
		add(a.cmdKey("Bloğu çoğalt"), "duplicate")
	case empty:
		add("/", "add a block")
		add("#", "heading")
		add("-", "list")
		add("[]", "task")
		add("```", "code")
	default:
		add(move, "move")
		add(a.cmdKey("Bloğu çoğalt"), "duplicate")
		add("⇧⌘8", "bullet list")
		add(a.cmdKey("Blok menüsü"), "block menu")
	}
	return out
}
