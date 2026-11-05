package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// A page whose text starts with a heading that is its title shows the title
// twice, over the text and in it. The heading in the text is not shown; it
// is kept aside, and written back as it was ahead of the text when the page
// is saved, so the file is never changed by it.

// splitTitle takes the first heading of md off when it is the title: the
// rest, and what was taken (the heading and the blank lines after it).
func splitTitle(md, title string) (rest, head string) {
	lines := strings.SplitAfter(md, "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return md, ""
	}
	line := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(line, "# ") || !strings.EqualFold(strings.TrimSpace(strings.TrimRight(line[2:], "# ")), strings.TrimSpace(title)) {
		return md, ""
	}
	j := i + 1
	for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
		j++
	}
	return strings.Join(lines[j:], ""), strings.Join(lines[:j], "")
}

// parsePage reads the Markdown of page p into the editor's text, without
// the heading that is its title.
func (a *app) parsePage(p *Page, md string) richtext.Text {
	rest, head := splitTitle(md, p.Title)
	if a.heads == nil {
		a.heads = map[*Page]string{}
	}
	if head == "" {
		delete(a.heads, p)
	} else {
		a.heads[p] = head
	}
	return a.withImages(p, a.parse(rest))
}

// pageMarkdown writes the editor's text of page p as its Markdown, with the
// heading that was kept aside ahead of it.
func (a *app) pageMarkdown(p *Page, t richtext.Text) string {
	body := bodyMarkdown(t)
	if head := a.heads[p]; head != "" {
		if strings.TrimSpace(body) == "" {
			return strings.TrimRight(head, "\n") + "\n"
		}
		return head + body
	}
	return body
}
