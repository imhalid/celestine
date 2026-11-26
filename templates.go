package main

import (
	"regexp"
	"slices"
	"strconv"
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
// palette and the sidebar's menu, and the / menu's Insert… puts one in a page
// as it is written. In a template, {{title}} is the page's title, {{date}}
// (or {{today}}) today as a day link, {{yesterday}} and {{tomorrow}} the days
// beside it, {{weekday}} today's name, {{time}} the time, {{cursor}} where the
// caret lands, and {{prompt:Name}} a value asked for as the template is used,
// once for each name however often it is written; the Turkish names,
// {{başlık}}, {{tarih}}, {{bugün}}, {{dün}}, {{yarın}}, {{gün}}, {{saat}},
// {{imleç}} and {{sor:Ad}}, are read as well (#537, #555).

// cursorMark stands for {{cursor}} between a template's expansion and the page it went into, where it is taken out
// and the caret put in its place: a rune of the private use area, which no text has.
const cursorMark = "\uE000"

// promptVar is a {{prompt:Name}} of a template, or {{sor:Ad}}: its name.
var promptVar = regexp.MustCompile(`\{\{\s*(?:prompt|sor)\s*:\s*([^}]*?)\s*\}\}`)

// templatePrompts are the names a template asks for, each once, in the order they first come.
func templatePrompts(body string) []string {
	var out []string
	for _, m := range promptVar.FindAllStringSubmatch(body, -1) {
		if name := m[1]; name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// expandTemplate fills in what a template says of the page and the day, the values asked for by their names (one
// not given is left empty), and {{cursor}} as cursorMark, which the page it goes into takes out (takeCursorMark).
func expandTemplate(body, title string, now time.Time, vals map[string]string) string {
	day := func(d time.Time) string { return "[[" + d.Format("2006-01-02") + "]]" }
	today, yesterday, tomorrow := day(now), day(now.AddDate(0, 0, -1)), day(now.AddDate(0, 0, 1))
	weekday := dayName(now.Weekday(), false)
	body = promptVar.ReplaceAllStringFunc(body, func(m string) string { return vals[promptVar.FindStringSubmatch(m)[1]] })
	return strings.NewReplacer(
		"{{title}}", title, "{{başlık}}", title, "{{baslik}}", title,
		"{{date}}", today, "{{today}}", today, "{{tarih}}", today, "{{bugün}}", today, "{{bugun}}", today,
		"{{yesterday}}", yesterday, "{{dün}}", yesterday, "{{dun}}", yesterday,
		"{{tomorrow}}", tomorrow, "{{yarın}}", tomorrow, "{{yarin}}", tomorrow,
		"{{weekday}}", weekday, "{{gün}}", weekday, "{{gun}}", weekday,
		"{{time}}", now.Format("15:04"), "{{saat}}", now.Format("15:04"),
		"{{cursor}}", cursorMark, "{{imleç}}", cursorMark, "{{imlec}}", cursorMark,
	).Replace(body)
}

// plainTemplate is a template expanded for a page no one is typing in, as an agent's: nothing is asked, and there is
// no caret to put.
func plainTemplate(body, title string, now time.Time) string {
	return strings.ReplaceAll(expandTemplate(body, title, now, nil), cursorMark, "")
}

// takeCursorMark takes the first cursorMark out of the open page and puts the caret there, and the rest out too.
// It reports whether there was one.
func (a *app) takeCursorMark() bool {
	if a.pane().doc == nil {
		return false
	}
	mark := []rune(cursorMark)[0]
	var at []int
	i := 0
	for _, r := range a.pane().doc.Text().String() {
		if r == mark {
			at = append(at, i)
		}
		i++
	}
	if len(at) == 0 {
		return false
	}
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		for k := len(at) - 1; k >= 0; k-- {
			ed.Replace(at[k], at[k]+1, richtext.Text{})
		}
		ed.Select(at[0], at[0])
	})
	a.pane().fresh, a.pane().refocus, a.pane().revealWant = false, true, true
	return true
}

// tplAsk is the values a template asks for, while its dialog is open: the names, what is typed for each, and what
// is done with them.
type tplAsk struct {
	open  bool
	names []string
	vals  []string
	then  func(map[string]string)
}

// askTemplate runs then with the values the template body asks for: at once when it asks for none, or once they
// are typed in its dialog. Cancel does nothing.
func (a *app) askTemplate(body string, then func(map[string]string)) {
	names := templatePrompts(body)
	if len(names) == 0 {
		then(nil)
		return
	}
	a.tplAsk = tplAsk{open: true, names: names, vals: make([]string, len(names)), then: then}
}

// tplAskView is the dialog of the values a template asks for, while it is open: a field for each, the first with
// the focus, and Enter in the last uses the template.
func (a *app) tplAskView(c *ui.Context) {
	q := &a.tplAsk
	if q.then == nil {
		return
	}
	done := func() {
		vals := map[string]string{}
		for i, n := range q.names {
			vals[n] = strings.TrimSpace(q.vals[i])
		}
		then := q.then
		*q = tplAsk{}
		then(vals)
	}
	ryugo.Dialog(c.Key("tpl-ask"), &q.open, ryugo.DialogOptions{Title: locales.TemplatesFillIn(), Width: 420, Footer: func() {
		ui.Spacer(c)
		if ryugo.Button(c.Key("tpl-ask-no"), locales.CommonCancel2(), ryugo.ButtonOptions{Size: ryugo.Sm, Variant: ryugo.Soft}).Clicked() {
			q.open = false
		}
		if ryugo.Button(c.Key("tpl-ask-yes"), locales.TemplatesUse(), ryugo.ButtonOptions{Size: ryugo.Sm, Color: ryugo.From(c).Colors.Accent, Variant: ryugo.Solid}).Clicked() {
			done()
		}
	}}, func() {
		cl := ryugo.From(c).Colors
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			for i, n := range q.names {
				ui.Text(c, n).FontSize(ryugo.FsSm).FontWeight(600).TextColor(cl.Text2).Margin(map[bool]float32{true: 6}[i > 0], 0, 0, 0)
				in := a.fieldMenu(ryugo.Input(c.Key("tpl-ask-"+strconv.Itoa(i)), &q.vals[i], ryugo.InputOptions{Label: n}), &q.vals[i], fieldOpts{oneLine: true})
				if i == 0 {
					in.AutoFocus()
				}
				if in.Submitted() && i == len(q.names)-1 {
					done()
					return
				}
			}
		})
	})
	if q.then != nil && !q.open { // closed by Cancel, Esc or the scrim: nothing is made
		*q = tplAsk{}
	}
}

// newPageFrom makes a page in c with the template's text, once the values it
// asks for are given, and opens it, its title ready to be typed, or the caret
// where the template says.
func (a *app) newPageFrom(c *Collection, parent *Page, tpl string) {
	if c.Dynamic() {
		a.newPage(c, parent)
		return
	}
	if c.Journal {
		a.openToday()
		return
	}
	a.askTemplate(tpl, func(vals map[string]string) {
		p, err := c.NewPage(parent, msg("common_untitled"))
		if err != nil {
			a.fail(err)
			return
		}
		a.noteCreated()
		if tpl != "" {
			if err := p.SaveText(expandTemplate(tpl, p.Title, a.clock(), vals)); err != nil {
				a.fail(err)
			}
		}
		a.open(p)
		a.freshTitle(a.pane())
		a.takeCursorMark()
	})
}

// templateMenu is the menu of a collection's templates, for a page from each.
func (a *app) templateMenu(c *Collection, parent *Page) []menuItem {
	var out []menuItem
	out = append(out, menuItem{title: "templates_blankPage", glyph: iconPage, run: func() { a.newPageFrom(c, parent, "") }})
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
	page := a.pane().page
	a.askTemplate(tpl.Body, func(vals map[string]string) {
		if a.pane().page != page || a.pane().doc == nil { // another page is open now
			return
		}
		a.insertExpanded(start, a.parse(expandTemplate(tpl.Body, page.Title, a.clock(), vals)))
		a.takeCursorMark()
	})
}

// insertExpanded puts text, a template filled in, in the open page at the line the caret is in, as insertTemplate.
func (a *app) insertExpanded(start int, text richtext.Text) {
	t := a.pane().doc.Text()
	ps, pe := t.ParagraphRange(min(start, t.Len()))
	n := text.Len()
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		if pe == ps {
			ed.Replace(ps, pe, text)
			ed.Select(ps+n, ps+n)
			return
		}
		ed.Replace(pe, pe, richtext.NewText("\n"))
		ed.Replace(pe+1, pe+1, text)
		ed.Select(pe+1+n, pe+1+n)
	})
	a.pane().doc.SetTypingAttributes(richtext.Attributes{})
}

// chooseTemplate takes the "/…" away and puts the template in.
func (a *app) chooseTemplate(tpl NamedTemplate) {
	s := &a.slash
	_, caret := a.pane().doc.Selection()
	start := s.start
	a.pane().doc.Edit(func(ed *richtext.Edit) {
		ed.Replace(start, max(caret, start), richtext.Text{})
		ed.Select(start, start)
	})
	s.open = false
	a.insertTemplate(start, tpl)
}

// templateItems are the collection's templates, as lines of the Ekle… menu.
func (a *app) templateItems(q string) []menuItem {
	if a.pane().page == nil {
		return nil
	}
	var out []menuItem
	for _, t := range a.pane().page.Coll.AllTemplates() {
		t := t
		if q != "" && !menuHas(q, "Şablon "+t.Name, locales.TemplatesTemplateLabel(t.Name)) {
			continue
		}
		out = append(out, menuItem{title: locales.TemplatesTemplateLabel(t.Name), sub: firstLine(t.Body), glyph: iconTemplate, group: "templates_group", run: func() { a.chooseTemplate(t) }})
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
		ui.Column(c.Key("cs-tpl:"+itoa(i+1))).FillWidth().Gap(6).Padding(10).Radius(rad(12)).Background(groupBg(cl)).Border(1, cl.WellBorder).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				a.fieldMenu(ryugo.Input(c.Key("cs-tpl-name"), &t.Name, ryugo.InputOptions{Label: locales.TemplatesTheTemplatesName(), Placeholder: locales.TemplatesMeetingNotes(), Size: ryugo.Sm}), &t.Name, fieldOpts{oneLine: true}).Grow(1)
				if ryugo.Button(c.Key("cs-tpl-del"), locales.TemplatesDeleteTemplate(), ryugo.ButtonOptions{Icon: iconTrash, IconOnly: true, Color: cl.Up, Variant: ryugo.Soft, Size: ryugo.Sm}).Clicked() {
					drop = i
				}
			})
			a.fieldMenu(ryugo.Textarea(c.Key("cs-tpl-body"), &t.Body, ryugo.TextareaOptions{Label: locales.TemplatesTheTemplatesText(), Placeholder: locales.TemplatesBodyPlaceholder(), Rows: 4, MaxRows: 12, Size: ryugo.Sm}), &t.Body, fieldOpts{})
		})
	}
	if drop >= 0 {
		d.Templates = append(d.Templates[:drop], d.Templates[drop+1:]...)
	}
	if ryugo.Button(c.Key("cs-tpl-add"), locales.TemplatesAddTemplate(), ryugo.ButtonOptions{Icon: iconPlus, Size: ryugo.Sm}).Clicked() {
		d.Templates = append(d.Templates, NamedTemplate{Name: locales.TemplatesNewTemplate()})
	}
}
