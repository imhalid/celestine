package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
)

// Links to a heading (#549): [[Page#Heading]] is a link to the page, which opens it at that heading. Its chip says
// "Page › Heading"; the [[ menu offers a page's headings once a "#" is typed after its title; the page counts it among
// its backlinks, and a rename of the page writes the link again with the heading kept. A title with a "#" of its own
// ("C#") is a page's title first: only a target no page is titled is split.

// splitHeading is a link's target as the page and the heading it names: at the first "#" with words on both sides,
// or the whole of it and no heading.
func splitHeading(target string) (page, heading string) {
	i := strings.Index(target, "#")
	if i <= 0 || strings.TrimSpace(target[i+1:]) == "" || strings.TrimSpace(target[:i]) == "" {
		return target, ""
	}
	return strings.TrimSpace(target[:i]), strings.TrimSpace(target[i+1:])
}

// headingsOf are the headings of page p, as their text reads, in order.
func headingsOf(p *Page) []string {
	body, err := p.Body()
	if err != nil {
		return nil
	}
	t := decodeBody(body)
	var out []string
	for ps, para := range t.Paragraphs() {
		if para.Style.Heading == 0 {
			continue
		}
		_, pe := t.ParagraphRange(ps)
		if h := strings.TrimSpace(strings.ReplaceAll(t.Slice(ps, pe).String(), string(richtext.ObjectReplacement), "")); h != "" {
			out = append(out, h)
		}
	}
	return out
}

// goToHeading puts the caret at the start of the open page's first heading that reads heading, any case, and the
// page there; it reports whether there is one.
func (a *app) goToHeading(heading string) bool {
	if a.doc == nil {
		return false
	}
	t := a.doc.Text()
	for ps, para := range t.Paragraphs() {
		if para.Style.Heading == 0 {
			continue
		}
		_, pe := t.ParagraphRange(ps)
		if strings.EqualFold(strings.TrimSpace(t.Slice(ps, pe).String()), heading) {
			a.gotoPos(ps)
			a.fresh = false
			return true
		}
	}
	return false
}

// headingLinks are the lines of the [[ menu for q typed as "Page#…": the headings of the page so titled that match
// what follows the "#", each a link to it; nil when q names no page and heading.
func (a *app) headingLinks(q string, start int) []menuItem {
	i := strings.Index(q, "#")
	if i <= 0 {
		return nil
	}
	p := a.pageTitled(strings.TrimSpace(q[:i]))
	if p == nil {
		return nil
	}
	hq := strings.TrimSpace(q[i+1:])
	var out []menuItem
	for _, h := range headingsOf(p) {
		if hq != "" && match(hq, h) < 0 {
			continue
		}
		target := p.Title + "#" + h
		out = append(out, menuItem{title: locales.LinksPageHeading(p.Title, h), glyph: iconPage, run: func() { a.insertChip(start, &chip{kind: "page", target: target}) }})
	}
	return out
}
