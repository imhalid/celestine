package main

import (
	"strings"
	"testing"
	"time"
)

func TestExpandTemplate(t *testing.T) {
	now := time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC)
	got := expandTemplate("# {{başlık}}\n{{tarih}} {{saat}} {{bugün}}", "Toplantı", now)
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
	c.Templates = []NamedTemplate{{"Toplantı", "# Toplantı {{tarih}}\n\n- [ ] karar\n"}, {"", "silinir"}}
	if got := c.allTemplates(); len(got) != 2 || got[0].Name != "Varsayılan" || got[1].Name != "Toplantı" {
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
	if err := c.setConfig(a.cs.draft.configJSON()); err != nil {
		t.Fatal(err)
	}
	if len(c.Templates) != 1 || c.Templates[0].Body != "## kayıtlı" {
		t.Fatalf("not taken: %+v", c.Templates)
	}
}
