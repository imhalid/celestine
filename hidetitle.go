package main

import (
	"strings"

	"github.com/egoist/mygo/ui/richtext"
)

// A page that comes in from outside (a Markdown file, an agent's text) may
// start with a heading that is its title, which would show twice, over the
// text and in it: that heading is taken off.

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

// parsePage reads the body of page p into the editor's text, its objects drawn.
func (a *app) parsePage(p *Page, body string) richtext.Text { return a.dress(p, decodeBody(body)) }
