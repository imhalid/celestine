package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Find and replace across the library (#553): the search, with a replace field. What is searched for, when it is
// plain text (no tag, property, collection or day in it), is replaced in every page that has it, or in one from its
// row, as the find bar of a page replaces: written as found, any case, when it is all small letters without marks,
// and as written when it has a capital or a mark (#212); in the text, not inside chips. Each page changed is kept as a
// version first, and the notice's Undo puts every one back as it was, unless it changed since.

// replaceState is the search's replace: whether its field shows, what is typed in it, and what the library has of
// the phrase searched for, as last counted.
type replaceState struct {
	on      bool
	focus   bool // its field is to take the focus, as it shows
	with    string
	phrase  string
	built   uint64
	pages   []*Page
	matches int
}

// replaceable is the phrase a search for q replaces, and whether there is one: q as typed, without the quotes of a
// phrase, when it asks for nothing but text.
func replaceable(q string) (string, bool) {
	q = strings.TrimSpace(q)
	if len(q) >= 2 && strings.HasPrefix(q, `"`) && strings.HasSuffix(q, `"`) {
		q = q[1 : len(q)-1]
	}
	if strings.TrimSpace(q) == "" || strings.ContainsAny(q, `:<>()"`) {
		return "", false
	}
	for _, w := range strings.Fields(q) {
		if strings.HasPrefix(w, "#") || strings.HasPrefix(w, "-") || strings.HasPrefix(w, "@") || w == "OR" {
			return "", false
		}
	}
	return q, true
}

// replaceInBody is body with every match of phrase replaced by with, as the find bar matches, outside chips, and how
// many there were.
func replaceInBody(body, phrase, with string) (string, int) {
	t := decodeBody(body)
	exact := findExact(phrase)
	hits := findIn(findRunes(t.String(), exact), findRunes(phrase, exact))
	if len(hits) == 0 {
		return body, 0
	}
	for k := len(hits) - 1; k >= 0; k-- {
		s, e := hits[k][0], hits[k][1]
		attrs := t.AttributesAt(s)
		attrs.Attachment = nil
		t = t.ReplaceString(s, e, with, attrs)
	}
	return encodeBody(t), len(hits)
}

// replaceCount is the pages of the library that have phrase, and how many times, counted again when the pages
// changed.
func (a *app) replaceCount(phrase string) ([]*Page, int) {
	r := &a.repl
	gen := PagesGen.Load() + uint64(a.links.built.UnixNano())
	if r.phrase == phrase && r.built == gen && r.pages != nil {
		return r.pages, r.matches
	}
	a.linksTo(nil, a.clock(), false)
	folded := foldText(phrase)
	r.phrase, r.built, r.pages, r.matches = phrase, gen, []*Page{}, 0
	for _, c := range a.lib.Collections {
		for _, p := range c.All() {
			if !strings.Contains(a.plainOf(p).folded, folded) {
				continue
			}
			body, err := p.Body()
			if err != nil || bodyReadable(body) != nil {
				continue
			}
			if _, n := replaceInBody(body, phrase, ""); n > 0 {
				r.pages = append(r.pages, p)
				r.matches += n
			}
		}
	}
	return r.pages, r.matches
}

// replaceIn replaces phrase with with in pages, each kept as a version first, and says how many it changed, with
// an Undo that puts them back.
func (a *app) replaceIn(pages []*Page, phrase, with string) {
	a.flush()
	var changes []linkChange
	total := 0
	for _, p := range pages {
		if a.unsaved[p] { // its text is not on disk: written over, it would be lost
			continue
		}
		body, err := p.Body()
		if err == nil {
			err = bodyReadable(body)
		}
		if err != nil {
			continue
		}
		nb, n := replaceInBody(body, phrase, with)
		if n == 0 {
			continue
		}
		if h, ok := historyOf(p); ok {
			h.SnapshotNow(p.File())
		}
		if err := p.Save(nb); err != nil {
			a.fail(err)
			continue
		}
		a.takeOutsideAll(p)
		changes = append(changes, linkChange{p, body, nb})
		total += n
	}
	a.linksStale()
	a.repl.pages = nil
	if total == 0 {
		return
	}
	a.say(toastOK, locales.ReplaceDone(total, len(changes)))
	a.noticeUndo = func() { a.undoRelink(changes) }
}

// replaceBar is the search's replace, under its field: a button that shows it, and while it shows, the field of
// what to put instead and the button that replaces in every page; nothing while the search is not plain text.
func (a *app) replaceBar(c *ui.Context) {
	r := &a.repl
	phrase, ok := replaceable(a.searchQ)
	if !ok {
		return
	}
	if !r.on {
		if ryugo.Button(c.Key("repl-on"), locales.ReplaceShow(), ryugo.ButtonOptions{Icon: iconEditing, Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			r.on, r.focus = true, true
		}
		return
	}
	pages, n := a.replaceCount(phrase)
	ui.Row(c.Key("repl")).Grow(1).MinWidth(0).Gap(8).AlignItems(ui.Center).Children(func() {
		in := a.fieldMenu(ryugo.Input(c.Key("repl-with"), &r.with, ryugo.InputOptions{Label: locales.ReplaceWith(), Placeholder: locales.ReplaceWith(), Size: ryugo.Sm}), &r.with, fieldOpts{oneLine: true}).Grow(1).MinWidth(0)
		if r.focus {
			in.Focus()
			r.focus = false
		}
		all := ryugo.Button(c.Key("repl-all"), locales.ReplaceInPages(n, len(pages)), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Solid, Color: ryugo.From(c).Colors.Accent})
		if n == 0 {
			all.Disabled(true)
		}
		if all.Clicked() && n > 0 {
			a.replaceIn(pages, phrase, r.with)
		}
		if a.iconButton(c, iconClose, locales.ReplaceHide()) {
			r.on = false
		}
	})
}

// replaceRowButton is a search result's own replace, while the search's replace shows: it replaces in that page.
func (a *app) replaceRowButton(c *ui.Context, p *Page) {
	phrase, ok := replaceable(a.searchQ)
	if !ok || !a.repl.on {
		return
	}
	if ryugo.Button(c.Key("repl-one"), locales.ReplaceHere(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
		a.later(func() { a.replaceIn([]*Page{p}, phrase, a.repl.with) })
	}
}
