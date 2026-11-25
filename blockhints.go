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
		add(a.cmdKey("zoomOut"), "zoomOut2")
	}
	anchor, caret := a.pane().doc.Selection()
	if anchor != caret { // a selection: its formats
		add("⌘B", "bold")
		add("⌘I", "italic")
		add(codeKey(), "code")
		add("⌘U", "underline")
		add("⇧⌘X", "strikethrough")
		return out
	}
	t := a.pane().doc.Text()
	st := t.ParagraphStyleAt(caret)
	ps, pe := t.ParagraphRange(caret)
	empty := strings.TrimSpace(t.Slice(ps, pe).String()) == ""
	move := keyPair(a.cmdKey("moveTheBlockUp"), a.cmdKey("moveTheBlockDown"))
	kids, folded := blockHasKids(t, caret), foldOf(t, caret) != nil
	zoomable := kids && !a.wholePage(t, caret) // zoomIn does not zoom into the whole page
	foldHint := func() {
		switch {
		case folded:
			add(a.cmdKey("foldUnfold"), "unfold")
		case kids:
			add(a.cmdKey("foldUnfold"), "fold")
		}
	}
	switch {
	case st.Heading > 0:
		if zoomable {
			add(a.cmdKey("zoomIntoTheBlock"), "zoomIntoTheSection")
		}
		foldHint()
		add("⌥⌘1–6", "headingLevel")
		add("⌥⌘0", "plainText2")
		add(move, "move2")
	case st.List != richtext.NoList && empty && st.Indent > 0: // ↵ on an empty item takes it a level out
		add("↵", "outdent")
	case st.List != richtext.NoList && empty: // and at the first level ends the list
		add("↵", "endTheList")
	case st.Quote && empty:
		add("↵", "endTheQuote")
	case st.List != richtext.NoList:
		add("Tab", "indent")
		add("⇧Tab", "outdent")
		if zoomable {
			add(a.cmdKey("zoomIntoTheBlock"), "zoomIntoTheItem")
		}
		foldHint()
		add(move, "move2")
	case st.CodeBlock && empty && ps > 0 && t.ParagraphStyleAt(ps-1).CodeBlock:
		add("↵", "endTheCode")
	case st.CodeBlock:
		add("Tab", "indent")
		add("⇧Tab", "outdent")
		add(move, "move2")
		add(a.cmdKey("duplicateTheBlock"), "duplicate")
	case empty:
		add("/", "addABlock")
		add("#", "heading")
		add("-", "list")
		add("[]", "task")
		add("```", "code")
	default:
		add(move, "move2")
		add(a.cmdKey("duplicateTheBlock"), "duplicate")
		add("⇧⌘8", "bulletList")
		add(a.cmdKey("common_blockMenu"), "blockMenu2")
	}
	return out
}

// wholePage reports whether the block the caret is in is the whole page. Only a block from the page's start can
// be; that one is walked once for each text, not in every frame the hints are drawn (#506).
func (a *app) wholePage(t richtext.Text, caret int) bool {
	if blockHeadOf(t, caret) > 0 {
		return false
	}
	if m := &a.wholeBlock; !m.ok || m.text != t {
		_, e := blockRange(t, 0)
		*m = wholeBlock{text: t, ok: true, whole: e >= t.Len()}
	}
	return a.wholeBlock.whole
}

// wholeBlock is whether the block at the start of a text is all of it (wholePage).
type wholeBlock struct {
	text      richtext.Text
	ok, whole bool
}
