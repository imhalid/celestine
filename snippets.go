package main

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Snippets (#554): abbreviations of one's own, kept in the settings, each with its text: ";sig" for a signature,
// ";mtg" for a meeting's outline. One typed as a word and ended by a space, a tab, Enter or a mark that ends a
// sentence is put out as its text; the first undo brings back what was typed. The text is Markdown, so a snippet of
// lines is lines, a list a list, and it says what a template does: {{date}}, {{time}}, {{cursor}} for where the
// caret goes, and the others (templates.go), with nothing asked.

// Snippet is an abbreviation and the text it is put out as.
type Snippet struct {
	Abbr string `json:"abbr"`
	Text string `json:"text"`
}

// snippetEnds are the marks besides white space that end an abbreviation typed. ";" and ":" are not among them:
// they start abbreviations.
const snippetEnds = ".,!?)"

// snippetFor is the text of the abbreviation abbr, written as it was kept, and whether there is one.
func snippetFor(list []Snippet, abbr string) (string, bool) {
	for _, s := range list {
		if s.Abbr != "" && s.Abbr == abbr {
			return s.Text, true
		}
	}
	return "", false
}

// expandSnippet puts out the abbreviation before the mark just typed at the caret as its text, and reports whether
// it did. typed is whether the text grew by what was typed, not an undo nor a paste.
func (a *app) expandSnippet(text richtext.Text, caret int, typed bool) bool {
	if !typed || caret < 2 || len(a.settings.Snippets) == 0 || a.page == nil {
		return false
	}
	end := []rune(text.Slice(caret-1, caret).String())
	if len(end) != 1 || !unicode.IsSpace(end[0]) && !strings.ContainsRune(snippetEnds, end[0]) {
		return false
	}
	ps, _ := text.ParagraphRange(caret - 1)
	if text.ParagraphStyleAt(ps).CodeBlock || text.AttributesAt(caret-2).Code {
		return false
	}
	ws := caret - 1
	for ws > ps && !unicode.IsSpace([]rune(text.Slice(ws-1, ws).String())[0]) {
		ws--
	}
	body, ok := snippetFor(a.settings.Snippets, text.Slice(ws, caret-1).String())
	if !ok || body == "" {
		return false
	}
	body = expandTemplate(body, a.page.Title, a.clock(), nil)
	var out richtext.Text
	if strings.Contains(body, "\n") { // lines: Markdown, as a template's
		out = a.parse(strings.TrimRight(body, "\n"))
	} else {
		attrs := text.AttributesAt(ws)
		attrs.Attachment = nil
		out = richtext.NewText(body)
		out = out.Format(0, out.Len(), func(at *richtext.Attributes) { *at = attrs }) // the look of where it is typed
	}
	n := out.Len()
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(ws, caret-1, out)
		ed.Select(ws+n+1, ws+n+1)
	})
	a.takeCursorMark()
	return true
}

// settingsSnippets is the list of snippets in the settings: each abbreviation and its text, a cross to take one
// away, and a button for another.
func (a *app) settingsSnippets(c *ui.Context) {
	list := &a.settings.Snippets
	changed, drop := false, -1
	settingGroup(c, "set-snippets", func() {
		settingRow(c, locales.SnippetsTitle(), locales.SnippetsHelp(), func() {})
		ui.Column(c).FillWidth().Gap(8).Padding(4, 0, 0, 0).Children(func() {
			for i := range *list {
				s := &(*list)[i]
				abbr, body := s.Abbr, s.Text
				ui.Row(c.Key("snip:" + strconv.Itoa(i))).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
					a.fieldMenu(ryugo.Input(c.Key("snip-abbr"), &abbr, ryugo.InputOptions{Label: locales.SnippetsAbbreviation(), Placeholder: ";sig", Size: ryugo.Sm}), &abbr, fieldOpts{oneLine: true}).Width(120).Shrink(0)
					a.fieldMenu(ryugo.Textarea(c.Key("snip-text"), &body, ryugo.TextareaOptions{Label: locales.SnippetsText(), Placeholder: locales.SnippetsTextPlaceholder(), Rows: 1, MaxRows: 6}), &body, fieldOpts{}).Grow(1).MinWidth(0)
					if a.iconButton(c, iconClose, locales.SnippetsRemove(abbr)) {
						drop = i
					}
				})
				if abbr = strings.Join(strings.Fields(abbr), ""); abbr != s.Abbr || body != s.Text { // no white space in one
					s.Abbr, s.Text, changed = abbr, body, true
				}
			}
			if ryugo.Button(c.Key("snip-add"), locales.SnippetsAdd(), ryugo.ButtonOptions{Icon: iconPlus, Size: ryugo.Sm, Variant: ryugo.Soft}).AlignSelf(ui.Start).Clicked() {
				*list = append(*list, Snippet{})
				changed = true
			}
		})
	})
	if drop >= 0 {
		*list = append((*list)[:drop], (*list)[drop+1:]...)
		changed = true
	}
	if changed {
		a.saveSettingsSoon()
	}
}
