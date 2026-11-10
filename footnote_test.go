package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/richtext"
)

// Footnotes come in from Markdown as references (chips) and notes (paragraphs), go back out as they came, are kept
// in the body, and a reference leads to its note (#308).
func TestFootnotes(t *testing.T) {
	md := "Bir cümle.[^1] Bir daha[^not], `kodda [^2] değil`.\n\n[^1]: Birinci not, **kalın**.\n[^not]: İkinci not.\n"
	tx := importMarkdown(md)
	var refs []string
	for _, r := range tx.Runs(0, tx.Len()) {
		if att := r.Attributes.Attachment; att != nil {
			if ch, ok := att.Data.(*chip); ok && ch.kind == "footnote" {
				refs = append(refs, ch.target)
			}
		}
	}
	if strings.Join(refs, ",") != "1,not" {
		t.Fatalf("references: %v", refs)
	}
	if !strings.Contains(tx.String(), "[^1]: Birinci not") {
		t.Fatalf("the note is not a paragraph of text: %q", tx.String())
	}
	if got := strings.TrimSpace(bodyMarkdown(tx)); got != strings.TrimSpace(md) {
		t.Fatalf("written back:\n%s\nwant\n%s", got, md)
	}
	if back := decodeBody(encodeBody(tx)); bodyMarkdown(back) != bodyMarkdown(tx) {
		t.Fatal("not kept in the body")
	}

	a, tt := newTestApp(t, md)
	tt.Frame()
	a.goToNote("not")
	_, c := a.doc.Selection()
	if s := a.doc.Text().Slice(c, c+6).String(); s != "İkinci" {
		t.Fatalf("the caret is not at the note: %q", s)
	}
	a.goToNote("yok")
	if !strings.Contains(a.notice, "yok") {
		t.Fatalf("no word of a note that is not there: %q", a.notice)
	}
	_ = richtext.Text{}
}

// / ▸ Footnote puts a reference at the caret, the next number, and its note at the end, the caret in it (#308).
func TestChooseFootnote(t *testing.T) {
	a, tt := newTestApp(t, "Bir cümle.[^1]\n\n[^1]: Birinci.\n")
	tt.Frame()
	s := a.doc.Text().String()
	at := utf8.RuneCountInString(s[:strings.Index(s, ".")]) // before the full stop
	a.doc.SetSelection(at, at)
	a.slash.start, a.slash.open = at, true
	a.chooseFootnote()
	tt.Frame()
	a.doc.SetText(a.doc.Text()) // as saved
	md := bodyMarkdown(a.doc.Text())
	if !strings.Contains(md, "Bir cümle[^2].[^1]") || !strings.HasSuffix(strings.TrimSpace(md), "[^1]: Birinci.\n[^2]:") {
		t.Fatalf("written as:\n%s", md)
	}
	_, c := a.doc.Selection()
	if c != a.doc.Text().Len() {
		t.Fatalf("the caret is at %d, not in the new note at the end (%d)", c, a.doc.Text().Len())
	}
}
