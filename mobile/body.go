package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/internal/library"
)

// A page's body is the library's document (library.Doc), the same JSON the desktop writes. The editor edits
// MyGo's rich text: textOf turns the one into the other and docOf back. What the rich text has no run for (a link
// to a page, a day, a table, a picture, a footnote) is an object of the document; here it rides through the editor
// as an attachment that carries the object itself and is written back as it came, so that a page written on the
// desktop loses nothing for having been opened on the phone.

// textOf is the document as the editor's text.
func textOf(d library.Doc) richtext.Text {
	out := make([]richtext.Paragraph, 0, len(d))
	for _, bp := range d {
		p := richtext.Paragraph{}
		if bp.S != nil {
			s := *bp.S
			p.Style = richtext.ParagraphStyle{Heading: s.Heading, List: richtext.ListKind(s.List), Checked: s.Checked,
				Indent: s.Indent, Align: ui.Align(s.Align), Quote: s.Quote, QuoteDepth: s.QuoteDepth, CodeBlock: s.CodeBlock, ID: s.ID}
		}
		for _, r := range bp.R {
			at := richtext.Attributes{Italic: r.I, Underline: r.U, Strikethrough: r.X, Code: r.C, Link: r.L, Tag: r.G, Weight: r.W}
			text := r.T
			if r.O != nil {
				o := *r.O
				at.Attachment = &richtext.Attachment{Name: objName(&o), Data: &o}
				text = string(richtext.ObjectReplacement)
			}
			p.Runs = append(p.Runs, richtext.Run{Text: text, Attributes: at})
		}
		out = append(out, p)
	}
	return richtext.TextOf(out...)
}

// docOf is the editor's text as the document.
func docOf(t richtext.Text) library.Doc {
	var d library.Doc
	for _, p := range t.Paragraphs() {
		bp := library.Para{}
		if p.Style != (richtext.ParagraphStyle{}) {
			s := p.Style
			bp.S = &library.ParaStyle{Heading: s.Heading, List: int(s.List), Checked: s.Checked, Indent: s.Indent,
				Align: int(s.Align), Quote: s.Quote, QuoteDepth: s.QuoteDepth, CodeBlock: s.CodeBlock, ID: s.ID}
		}
		for _, r := range p.Runs {
			a := r.Attributes
			if at := a.Attachment; at != nil {
				if o, ok := at.Data.(*library.Obj); ok {
					obj := *o
					bp.R = append(bp.R, library.Run{O: &obj})
				}
				continue
			}
			run := library.Run{T: r.Text, I: a.Italic, U: a.Underline, X: a.Strikethrough, C: a.Code, L: a.Link, G: a.Tag}
			if a.Weight != 0 && a.Weight != 400 {
				run.W = a.Weight
			}
			bp.R = append(bp.R, run)
		}
		d = append(d, bp)
	}
	return d
}

// objName is what an object shows as in the text: the page or the day it links to, a picture's words.
func objName(o *library.Obj) string {
	switch {
	case o.Alias != "":
		return o.Alias
	case o.Target != "":
		return o.Target
	case o.Alt != "":
		return o.Alt
	case o.K == "table":
		return "▦"
	case o.K == "footnote":
		return "†"
	}
	return "•"
}
