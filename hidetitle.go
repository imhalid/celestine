package main

import (
	"github.com/egoist/mygo/ui/richtext"
)

// A page that comes in from outside (a Markdown file, an agent's text) may
// start with a heading that is its title, which would show twice, over the
// text and in it: that heading is taken off.

// parsePage reads the body of page p into the editor's text, its objects drawn.
func (a *app) parsePage(p *Page, body string) richtext.Text { return a.dress(p, decodeBody(body)) }
