package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

func TestExpandTemplate(t *testing.T) {
	now := time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC)
	got := expandTemplate("# {{başlık}}\n{{tarih}} {{saat}} {{bugün}}", "Toplantı", now, nil)
	if got != "# Toplantı\n[[2026-11-05]] 14:30 [[2026-11-05]]" {
		t.Fatalf("%q", got)
	}
}

// TestTemplates: a collection's templates make pages, from the collection's
// menu, the palette and the / menu; the settings keep them.
func TestTemplates(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1100, 800)
	c := a.page.Coll
	c.Template = "## Varsayılan {{başlık}}\n"
	c.Templates = []NamedTemplate{{Name: "Toplantı", Body: "# Toplantı {{tarih}}\n\n- [ ] karar\n"}, {Name: "", Body: "silinir"}}
	if got := c.AllTemplates(); len(got) != 2 || got[0].Name != "Varsayılan" || got[1].Name != "Toplantı" {
		t.Fatalf("%+v", got)
	}
	// the palette
	var cmd *palItem
	for _, it := range a.commands() {
		it := it
		if strings.HasPrefix(it.title, "Şablondan yeni") && strings.Contains(it.title, "Toplantı") {
			cmd = &it
		}
	}
	if cmd == nil {
		t.Fatal("no command for the template")
	}
	cmd.run()
	tt.Advance(200 * time.Millisecond)
	body, _ := a.page.Text()
	if !strings.Contains(body, "# Toplantı [[2026-") || !strings.Contains(body, "- [ ] karar") {
		t.Fatalf("page from the template: %q", body)
	}
	// from the / menu, in a page
	a.fresh, a.refocus = false, true
	a.doc.SetSelection(0, 0)
	tt.Frame()
	tt.Frame()
	typeEach(tt, "/")
	typeEach(tt, "toplant")
	tt.Advance(100 * time.Millisecond)
	if !tt.HasText("Şablon: Toplantı") {
		t.Fatalf("not in the menu: %v", tt.Texts())
	}
}

// TestTemplateSettings: the settings add, name and take away templates, and the file keeps them.
func TestTemplateSettings(t *testing.T) {
	a, tt := newTestApp(t, "x\n")
	tt.SetSize(1100, 900)
	c := a.page.Coll
	a.openCollSettings(c, 3)
	tt.Advance(600 * time.Millisecond)
	if err := tt.Click("Şablon ekle"); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if len(a.cs.draft.Templates) != 1 {
		t.Fatalf("%+v", a.cs.draft.Templates)
	}
	a.cs.draft.Templates[0].Body = "## kayıtlı"
	if err := c.SetConfig(a.cs.draft.ConfigJSON()); err != nil {
		t.Fatal(err)
	}
	if len(c.Templates) != 1 || c.Templates[0].Body != "## kayıtlı" {
		t.Fatalf("not taken: %+v", c.Templates)
	}
}

// TestTemplateVariables: the days beside today, its name, {{cursor}} as its mark, and the values asked for by name,
// each name once, a value not given left empty (#555).
func TestTemplateVariables(t *testing.T) {
	now := time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC) // a Thursday
	got := expandTemplate("{{yesterday}} {{tomorrow}} {{dün}} {{yarın}} {{weekday}}|{{cursor}}|{{prompt:Who}}/{{ sor : Who }}/{{prompt:Where}}", "T", now, map[string]string{"Who": "Ada"})
	want := "[[2026-11-04]] [[2026-11-06]] [[2026-11-04]] [[2026-11-06]] " + dayName(time.Thursday, false) + "|" + cursorMark + "|Ada/Ada/"
	if got != want {
		t.Fatalf("%q, want %q", got, want)
	}
	if names := templatePrompts("{{prompt:B}} {{sor:A}} {{prompt:B}} {{prompt:}}"); strings.Join(names, ",") != "B,A" {
		t.Fatalf("the names asked for: %v", names)
	}
	if got := plainTemplate("a{{cursor}}b {{prompt:X}}", "T", now); got != "ab " {
		t.Fatalf("for an agent: %q", got)
	}
}

// TestTemplateAsksAndPlacesTheCaret: a template with {{prompt:…}} asks for the values in a dialog before the page is
// made, and the page has them, with the caret where {{cursor}} was and no mark left; Cancel makes nothing (#555).
func TestTemplateAsksAndPlacesTheCaret(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1100, 800)
	c := a.page.Coll
	tpl := "Author: {{prompt:Author}}\n\n{{cursor}}\n\nBy {{prompt:Author}}\n"
	before := len(c.Pages)
	a.newPageFrom(c, nil, tpl)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	if !a.tplAsk.open || len(c.Pages) != before {
		t.Fatalf("no dialog (%v), or a page made before it: %d pages", a.tplAsk.open, len(c.Pages))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(300 * time.Millisecond)
	if a.tplAsk.open || len(c.Pages) != before {
		t.Fatalf("cancelled: open %v, %d pages", a.tplAsk.open, len(c.Pages))
	}
	a.newPageFrom(c, nil, tpl)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Type("Herbert")
	tt.Key(0, ui.KeyEnter)
	tt.Advance(300 * time.Millisecond)
	if len(c.Pages) != before+1 || a.page == nil {
		t.Fatalf("no page made: %d", len(c.Pages))
	}
	text := a.doc.Text().String()
	if strings.Count(text, "Herbert") != 2 || strings.Contains(text, cursorMark) {
		t.Fatalf("the page: %q", text)
	}
	_, caret := a.doc.Selection() // in the empty paragraph between the two lines, where {{cursor}} was alone
	if want := strings.Index(text, "\n\n") + 1; caret != len([]rune(text[:want])) || a.fresh {
		t.Fatalf("the caret at %d (want %d), the title to type %v, in %q", caret, len([]rune(text[:want])), a.fresh, text)
	}
}

// TestNewPageSplitButton: a collection with templates has one control in its head, the new page and an arrow with
// the pages of the other templates (#351): the arrow opens the templates' menu under it, the action makes a page.
func TestNewPageSplitButton(t *testing.T) {
	a, tt := newTestApp(t, "metin\n")
	tt.SetSize(1100, 800)
	c := a.page.Coll
	c.Templates = []NamedTemplate{{Name: "Toplantı", Body: "# Toplantı\n"}}
	if err := tt.Click("Koleksiyon: " + c.Name); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click(locales.ViewsNewPageFromTemplate()); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText("Toplantı") || !a.ctx.open {
		t.Fatalf("the arrow did not open the templates' menu: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Advance(200 * time.Millisecond)
	n := len(c.All())
	if err := tt.Click(newPageLabel(c)); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if len(c.All()) != n+1 {
		t.Fatalf("the action made no page: %d, was %d", len(c.All()), n)
	}
}
