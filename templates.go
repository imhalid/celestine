package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// Templates: a collection starts its new pages with the Markdown of its
// template (the first, which it always had), and may have more, each a name
// and a text: a page from one is a click away in the collection, in the
// palette and the sidebar's menu, and the / menu's Ekle… puts one in a page
// as it is written. In a template, {{başlık}} is the page's title, {{tarih}}
// (or {{bugün}}) today as a day link, {{saat}} the time.

// expandTemplate fills in what a template says of the page and the day.
func expandTemplate(body, title string, now time.Time) string {
	return strings.NewReplacer(
		"{{başlık}}", title, "{{baslik}}", title,
		"{{tarih}}", "[["+now.Format("2006-01-02")+"]]", "{{bugün}}", "[["+now.Format("2006-01-02")+"]]", "{{bugun}}", "[["+now.Format("2006-01-02")+"]]",
		"{{saat}}", now.Format("15:04"),
	).Replace(body)
}

// newPageFrom makes a page in c with the template's text, and opens it, its
// title ready to be typed.
func (a *app) newPageFrom(c *Collection, parent *Page, tpl string) {
	if c.Dynamic() {
		a.newPage(c, parent)
		return
	}
	if c.Journal {
		a.openToday()
		return
	}
	p, err := c.NewPage(parent, "Adsız")
	if err != nil {
		a.fail(err)
		return
	}
	if tpl != "" {
		if err := p.SaveText(expandTemplate(tpl, p.Title, a.clock())); err != nil {
			a.fail(err)
		}
	}
	a.open(p)
	a.fresh = true
}

// templateMenu is the menu of a collection's templates, for a page from each.
func (a *app) templateMenu(c *Collection, parent *Page) []menuItem {
	var out []menuItem
	out = append(out, menuItem{title: "Boş sayfa", glyph: iconPage, run: func() { a.newPageFrom(c, parent, "") }})
	for _, t := range c.AllTemplates() {
		t := t
		out = append(out, menuItem{title: t.Name, sub: firstLine(t.Body), glyph: iconTemplate, run: func() { a.newPageFrom(c, parent, t.Body) }})
	}
	return out
}

func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if ln = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(ln), "#-*> ")); ln != "" {
			if r := []rune(ln); len(r) > 44 {
				return string(r[:44]) + "…"
			}
			return ln
		}
	}
	return ""
}

var iconTemplate = icon(`<rect x="4.5" y="4.5" width="15" height="15" rx="2.5"/><path d="M4.5 9.5h15M9.5 9.5v10"/>`)

// insertTemplate puts a template's text in the open page at the line the
// caret is in (the empty line, or under the line with text).
func (a *app) insertTemplate(start int, tpl NamedTemplate) {
	text := a.parse(expandTemplate(tpl.Body, a.page.Title, a.clock()))
	t := a.doc.Text()
	ps, pe := t.ParagraphRange(min(start, t.Len()))
	n := text.Len()
	a.doc.Edit(func(ed *richtext.Edit) {
		if pe == ps {
			ed.Replace(ps, pe, text)
			ed.Select(ps+n, ps+n)
			return
		}
		ed.Replace(pe, pe, richtext.NewText("\n"))
		ed.Replace(pe+1, pe+1, text)
		ed.Select(pe+1+n, pe+1+n)
	})
	a.doc.SetTypingAttributes(richtext.Attributes{})
}

// chooseTemplate takes the "/…" away and puts the template in.
func (a *app) chooseTemplate(tpl NamedTemplate) {
	s := &a.slash
	_, caret := a.doc.Selection()
	start := s.start
	a.doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.Select(start, start)
	})
	s.open = false
	a.insertTemplate(start, tpl)
}

// templateItems are the collection's templates, as lines of the Ekle… menu.
func (a *app) templateItems(q string) []menuItem {
	if a.page == nil {
		return nil
	}
	var out []menuItem
	for _, t := range a.page.Coll.AllTemplates() {
		t := t
		if q != "" && match(q, "Şablon "+t.Name) < 0 {
			continue
		}
		out = append(out, menuItem{title: "Şablon: " + t.Name, sub: firstLine(t.Body), glyph: iconTemplate, group: "Şablonlar", run: func() { a.chooseTemplate(t) }})
	}
	return out
}

// csTemplates are the collection's templates in its settings: the first,
// and the ones with names.
func (a *app) csTemplates(c *ui.Context) {
	d := &a.cs.draft
	cl := ryugo.From(c).Colors
	ui.Text(c, locales.TemplatesOtherTemplates()).FontSize(ryugo.FsSm).FontWeight(620).TextColor(cl.Text)
	ui.Text(c, locales.TemplatesEachChosenByName()).FontSize(ryugo.FsSm).TextColor(cl.Muted).Wrap()
	drop := -1
	for i := range d.Templates {
		i := i
		t := &d.Templates[i]
		ui.Column(c.Key("cs-tpl:"+itoa(i+1))).FillWidth().Gap(6).Padding(10).Radius(rad(12)).Background(cl.Well).Border(1, cl.WellBorder).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("cs-tpl-name"), &t.Name, ryugo.InputOptions{Label: locales.TemplatesTheTemplatesName(), Placeholder: locales.TemplatesMeetingNotes(), Size: ryugo.Sm}), &t.Name, fieldOpts{oneLine: true}).Grow(1)
				if ryugo.Button(c.Key("cs-tpl-del"), locales.TemplatesDeleteTemplate(), ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: cl.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
					drop = i
				}
			})
			a.fieldMenu(ryugo.Textarea(c.Key("cs-tpl-body"), &t.Body, ryugo.TextareaOptions{Label: locales.TemplatesTheTemplatesText(), Placeholder: "## {{başlık}}\n\n- [ ] ", Rows: 4, MaxRows: 12, Size: ryugo.Sm}), &t.Body, fieldOpts{})
		})
	}
	if drop >= 0 {
		d.Templates = append(d.Templates[:drop], d.Templates[drop+1:]...)
	}
	if ryugo.Button(c.Key("cs-tpl-add"), locales.TemplatesAddTemplate(), ryugo.ButtonOptions{Icon: iconPlus, Size: ryugo.Sm}).Clicked() {
		d.Templates = append(d.Templates, NamedTemplate{Name: "Yeni şablon"})
	}
}
