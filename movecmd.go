package main

import (
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// The keyboard's side of dragging: the commands that put a page or a collection somewhere else, which the
// sidebar does with the pointer.

// siblingsOf are the pages that p is in a row with.
func siblingsOf(p *Page) []*Page {
	if p.Parent != nil {
		return p.Parent.Children
	}
	return p.Coll.Pages
}

// moveCommands are the commands that move the page that is open, and the collection that is shown.
func (a *app) moveCommands(cmd func(title, keys string, glyph *ui.SVG, run func()) palItem) []palItem {
	var out []palItem
	if p := a.page; p != nil && !a.noPage() && !p.Coll.Journal {
		done := func(what string) {
			a.linksStale()
			p.Coll.Open = true
			a.say(toastOK, what)
		}
		sib := siblingsOf(p)
		at := slices.Index(sib, p)
		up := cmd("moveThePageUpInIts", "", iconBack, func() {
			if err := a.lib.MovePage(p, p.Coll, p.Parent, at-1); err != nil {
				a.fail(err)
				return
			}
			done(locales.MoveUpDone())
		})
		if at <= 0 {
			up.off = locales.MoveAtTop()
		}
		down := cmd("moveThePageDownInIts", "", iconChevron, func() {
			if err := a.lib.MovePage(p, p.Coll, p.Parent, at+2); err != nil {
				a.fail(err)
				return
			}
			done(locales.MoveDownDone())
		})
		if at < 0 || at >= len(sib)-1 {
			down.off = locales.MoveAtBottom()
		}
		out = append(out, up, down)
		if p.Parent != nil {
			out = append(out, cmd("takeThePageOutOfIts", "", iconBack, func() {
				parent := p.Parent
				idx := slices.Index(siblingsOf(parent), parent) + 1
				if err := a.lib.MovePage(p, p.Coll, parent.Parent, idx); err != nil {
					a.fail(err)
					return
				}
				done(locales.MoveOutDone())
			}))
		}
		for _, c := range a.lib.Collections {
			c := c
			if c == p.Coll || c.Dynamic() || c.Journal {
				continue
			}
			out = append(out, cmd("moveThePage: "+c.Name, "", iconStack, func() {
				a.flush()
				if err := a.lib.MovePage(p, c, nil, len(c.Pages)); err != nil {
					a.fail(err)
					return
				}
				c.Open = true
				done(locales.MoveToCollection(c.Name))
			}))
		}
	}
	if c := a.coll; c != nil {
		out = append(out,
			cmd("moveTheCollectionUp", "", iconBack, func() {
				if err := a.lib.MoveCollection(c, -1); err != nil {
					a.fail(err)
				}
			}),
			cmd("moveTheCollectionDown", "", iconChevron, func() {
				if err := a.lib.MoveCollection(c, 1); err != nil {
					a.fail(err)
				}
			}))
	}
	return out
}
