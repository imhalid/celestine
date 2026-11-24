package main

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Extracting to a page (#550): the blocks of the selection, or the block the caret is in, become a page of their
// own, and a link to it takes their place. It is how a long note is split. The new page is titled from the first
// line; a heading there is the title, and leaves the body. It goes under the page where its collection has pages
// under pages, and beside it where not; a day of the journal has none, and its blocks go to the collection quick
// capture writes to. The pictures and files the blocks show go with them. Undo puts the blocks back in one step;
// the new page stays, to be trashed if it is not wanted.

// extractTitleMax is how many letters of the first line a page made from blocks takes as its title.
const extractTitleMax = 60

// extractRange is what extracting takes: from the start of the paragraph the selection starts in to the end of the
// one it ends in, or the block the caret is in (a heading with what is under it), and whether there is anything.
func extractRange(t richtext.Text, anchor, caret int) (s, e int, ok bool) {
	lo, hi := min(anchor, caret), max(anchor, caret)
	if lo == hi {
		s, e = blockRange(t, lo)
	} else {
		s, _ = t.ParagraphRange(lo)
		_, e = t.ParagraphRange(max(hi-1, lo))
	}
	return s, e, strings.TrimSpace(t.Slice(s, e).String()) != ""
}

// extractSplit is the title of a page made from part, and its body: the first line with words, cut to
// extractTitleMax letters, and part without it when it is a heading.
func extractSplit(part richtext.Text) (title string, body richtext.Text) {
	body = part
	for ps, p := range part.Paragraphs() {
		_, pe := part.ParagraphRange(ps)
		line := strings.TrimSpace(strings.ReplaceAll(part.Slice(ps, pe).String(), string(richtext.ObjectReplacement), ""))
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > extractTitleMax {
			line = strings.TrimSpace(string(r[:extractTitleMax])) + "…"
		}
		if p.Style.Heading > 0 {
			body = part.Slice(min(pe+1, part.Len()), part.Len())
		}
		return line, body
	}
	return "", body
}

// extractToPage makes a page of the blocks of the selection, or of the block the caret is in, and puts a link to it
// in their place.
func (a *app) extractToPage() {
	p := a.page
	if p == nil || a.noPage() || a.doc == nil {
		return
	}
	t := a.doc.Text()
	anchor, caret := a.doc.Selection()
	s, e, ok := extractRange(t, anchor, caret)
	if !ok {
		a.say(toastWarn, locales.ExtractNothing())
		return
	}
	title, body := extractSplit(t.Slice(s, e))
	if title == "" {
		title = msg("common_untitled")
	}
	coll, parent := p.Coll, p.Parent
	if coll.SubPages {
		parent = p
	}
	if coll.Journal || coll.Dynamic() {
		coll, parent = a.quickCollection(), nil
	}
	if coll == nil {
		a.say(toastWarn, locales.ExtractNowhere())
		return
	}
	np, err := coll.NewPage(parent, title)
	if err != nil {
		a.fail(err)
		return
	}
	saved := encodeBody(body)
	if err := np.Save(saved); err != nil {
		a.fail(err)
		return
	}
	a.carryFiles(p, np, saved)
	a.noteCreated()
	attrs := t.AttributesAt(s)
	attrs.Attachment = a.attachment(&chip{kind: "page", target: np.Title})
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.FormatParagraphs(s, s, func(st *richtext.ParagraphStyle) { *st = richtext.ParagraphStyle{} })
		ed.ReplaceString(s, e, string(richtext.ObjectReplacement), attrs)
		ed.Select(s+1, s+1)
	})
	a.linksStale()
	a.refocus = true
	a.say(toastOK, locales.ExtractDone(np.Title))
}

// carryFiles copies to page to the files beside page from that body names: the pictures and files of the blocks
// that went to it. They stay beside from too, for the undo that brings the blocks back.
func (a *app) carryFiles(from, to *Page, body string) {
	fsys := from.FS()
	entries, err := fsys.ReadDir(from.Dir)
	if err != nil {
		return
	}
	for _, en := range entries {
		name := en.Name()
		if en.IsDir() || strings.HasPrefix(name, ".") || !strings.Contains(body, name) {
			continue
		}
		data, err := fsys.ReadFile(filepath.Join(from.Dir, name))
		if err == nil {
			err = to.FS().WriteFile(filepath.Join(to.Dir, name), data)
		}
		if err != nil {
			a.fail(err)
		}
	}
}
