package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Bulk actions: in a collection's list and table, "Seç" turns the rows into
// ones to be chosen (a click marks, not opens), and a bar over them does the
// same to all the chosen: a tag, a move to another collection, a value of a
// choice, the trash.

type bulkState struct {
	on      bool
	sel     map[*Page]bool
	tagging bool   // the field for a tag is open
	tag     string // what is typed in it
	focus   bool
}

func (a *app) bulkClear() { a.bulk = bulkState{} }

func (a *app) bulkToggle(p *Page) {
	if a.bulk.sel == nil {
		a.bulk.sel = map[*Page]bool{}
	}
	if a.bulk.sel[p] {
		delete(a.bulk.sel, p)
	} else {
		a.bulk.sel[p] = true
	}
}

// bulkPages are the chosen pages among pages, in their order.
func (a *app) bulkPages(pages []*Page) []*Page {
	var out []*Page
	for _, p := range pages {
		if a.bulk.sel[p] {
			out = append(out, p)
		}
	}
	return out
}

// bulkMark is the box at the start of a row that shows whether it is chosen.
func (a *app) bulkMark(c *ui.Context, p *Page) {
	if !a.bulk.on {
		return
	}
	on := a.bulk.sel[p]
	box := ryugo.Checkbox(c.Key("mark"), &on, "", ryugo.CheckOptions{}).Shrink(0).Margin(0, 10, 0, 0)
	if box.Clicked() {
		a.bulkToggle(p)
	}
}

// bulkRowClicked says a click on the row of p: when rows are being chosen it
// marks the page, and reports true, so that the page is not opened.
func (a *app) bulkRowClicked(p *Page, clicked bool) bool {
	if !a.bulk.on || !clicked {
		return false
	}
	a.bulkToggle(p)
	return true
}

// bulkBar is the bar over the rows while they are being chosen.
func (a *app) bulkBar(c *ui.Context, coll *Collection, pages []*Page) {
	if !a.bulk.on {
		return
	}
	cl := ryugo.From(c).Colors
	chosen := a.bulkPages(pages)
	ui.Row(c.Key("bulk-bar")).FillWidth().Gap(8).Padding(6, 10).Radius(rad(12)).AlignItems(ui.Center).Wrap().Background(cl.Accent.Alpha(0.1)).Border(1, cl.Accent.Alpha(0.3)).Children(func() {
		msg := locales.BulkSelectedCount(len(chosen))
		if len(chosen) == 0 {
			msg = locales.BulkClickRows()
		}
		ui.Text(c, msg).FontSize(13).FontWeight(650).TextColor(cl.Text).Padding(0, 6)
		if ryugo.Button(c.Key("bulk-all"), locales.CommonSelectAll(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			for _, p := range pages {
				if a.bulk.sel == nil {
					a.bulk.sel = map[*Page]bool{}
				}
				a.bulk.sel[p] = true
			}
		}
		ui.Spacer(c)
		off := len(chosen) == 0
		if a.bulk.tagging {
			in := a.fieldMenu(ryugo.Input(c.Key("bulk-tag"), &a.bulk.tag, ryugo.InputOptions{Label: locales.BulkTag(), Placeholder: locales.BulkTagPlaceholder(), PrefixIcon: iconTag, Size: ryugo.Sm}), &a.bulk.tag, fieldOpts{oneLine: true}).Width(150)
			if a.bulk.focus {
				in.Focus()
				a.bulk.focus = false
			}
			if in.Submitted() || ryugo.Button(c.Key("bulk-tag-ok"), locales.CommonAdd(), ryugo.ButtonOptions{Color: cl.Accent, Variant: ryugo.Solid, Size: ryugo.Sm}).Clicked() {
				a.bulkAddTag(chosen, a.bulk.tag)
				a.bulk.tagging, a.bulk.tag = false, ""
			}
		} else if ryugo.Button(c.Key("bulk-tagb"), locales.BulkAddTag(), ryugo.ButtonOptions{Icon: iconTag, Size: ryugo.Sm, Disabled: off}).Clicked() {
			a.bulk.tagging, a.bulk.focus = true, true
		}
		if ryugo.Button(c.Key("bulk-move"), locales.BlocksMove(), ryugo.ButtonOptions{Icon: iconMoveDown, Size: ryugo.Sm, Disabled: off}).Clicked() {
			ww, _ := c.Size()
			a.showContext(ww-420, 230, a.bulkMoveMenu(chosen))
		}
		if choices := a.bulkChoices(coll, chosen); len(choices) > 0 {
			if ryugo.Button(c.Key("bulk-prop"), locales.BlocksSetProperty(), ryugo.ButtonOptions{Icon: iconSettings, Size: ryugo.Sm, Disabled: off}).Clicked() {
				ww, _ := c.Size()
				a.showContext(ww-360, 230, choices)
			}
		}
		if ryugo.Button(c.Key("bulk-trash"), locales.BlocksThrowAway(), ryugo.ButtonOptions{Icon: iconTrash, Color: cl.Up, Variant: ryugo.Soft, Size: ryugo.Sm, Disabled: off}).Clicked() {
			a.bulkTrash(chosen)
		}
		if ryugo.Button(c.Key("bulk-done"), locales.BulkDone(), ryugo.ButtonOptions{Size: ryugo.Sm}).Clicked() {
			a.bulkClear()
		}
	})
}

// bulkAddTag puts #tag at the end of each page that does not have it.
func (a *app) bulkAddTag(pages []*Page, tag string) {
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "#")
	if tag == "" || !tagQuery.MatchString(tag) || !tagWord.MatchString(" #"+tag) {
		a.say(toastWarn, locales.ATagStartsWithALetter())
		return
	}
	n := 0
	for _, p := range pages {
		if p == a.page {
			a.flush()
		}
		text, err := p.Text()
		if err != nil {
			continue
		}
		have := false
		for _, t := range tagsOf(text) {
			have = have || tagKey(t) == tagKey(tag)
		}
		if have {
			continue
		}
		body, err := p.Body()
		if err == nil {
			err = bodyReadable(body) // appendLine would make the page of the tag line alone (#143)
		}
		if err != nil {
			a.fail(err)
			continue
		}
		if err := p.Save(appendLine(body, "#"+tag)); err != nil {
			a.fail(err)
			continue
		}
		a.takeOutside(p)
		n++
	}
	a.linksStale()
	a.say(toastOK, locales.BulkTagAdded(n, tag))
}

// bulkMoveMenu lists the collections the pages can go to.
func (a *app) bulkMoveMenu(pages []*Page) []menuItem {
	var out []menuItem
	for _, co := range a.lib.Collections {
		co := co
		if co.Dynamic() || co.Journal {
			continue
		}
		out = append(out, menuItem{title: co.Name, glyph: collGlyph(co), run: func() { a.later(func() { a.bulkMove(pages, co) }) }})
	}
	return out
}

// bulkMove puts the pages at the end of the top of collection to.
func (a *app) bulkMove(pages []*Page, to *Collection) {
	n := 0
	for _, p := range pages {
		if p == a.page {
			a.flush()
		}
		if p.Coll == to && p.Parent == nil {
			continue
		}
		if err := a.lib.MovePage(p, to, nil, len(to.Pages)); err != nil {
			a.fail(err)
			continue
		}
		n++
	}
	a.linksStale()
	a.bulkClear()
	a.say(toastOK, locales.BulkMovedTo(n, to.Name))
}

// bulkChoices are the values of the collection's choices, as lines to give
// them to the pages.
func (a *app) bulkChoices(coll *Collection, pages []*Page) []menuItem {
	var out []menuItem
	for i := range coll.Properties {
		pr := &coll.Properties[i]
		if pr.Type != PropChoice {
			continue
		}
		for _, o := range pr.Options {
			id, o, name := pr.ID, o, pr.Name
			out = append(out, menuItem{title: name + ": " + o, glyph: iconSettings, run: func() {
				for _, p := range pages {
					if p.Coll == coll {
						a.setValue(p, id, o)
					}
				}
			}})
		}
	}
	return out
}

// bulkTrash moves the pages to the trash.
func (a *app) bulkTrash(pages []*Page) {
	a.flush() // what is written is saved before the pages go: not after, into a folder that is not there
	n := 0
	for _, p := range pages {
		if _, err := a.lib.TrashPage(p); err == nil {
			n++
			a.dropFromPanes(func(q *Page) bool { return pageUnder(q, p) }, func(*Collection) bool { return false })
			if pageUnder(a.page, p) {
				a.page, a.doc = nil, nil
			}
		}
	}
	a.linksStale()
	a.bulkClear()
	a.say(toastOK, locales.BulkMovedToTrash(n))
}
