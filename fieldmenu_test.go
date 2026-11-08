package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A right click on a property's value, on its name and on a field of Ryugo's opens the app's menu,
// not the system's; its Yapıştır puts the clipboard in the field.
func TestFieldsOpenTheAppMenu(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1100, 700)
	tt.Advance(300 * time.Millisecond)
	coll := a.page.Coll
	coll.AddProperty("Yazar", PropText, nil)
	tt.Advance(300 * time.Millisecond)
	a.clipRead = func() string { return "Herbert" }
	open := func(label string) []menuItem {
		t.Helper()
		a.ctx.open = false
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q: %v", label, tt.Texts())
		}
		tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		if !a.ctx.open || tt.Menu() != nil {
			t.Fatalf("%q: app menu %v, system menu %v", label, a.ctx.open, tt.Menu() != nil)
		}
		return a.ctx.stack[0]
	}
	has := func(items []menuItem, title string) *menuItem {
		for i := range items {
			if items[i].title == title {
				return &items[i]
			}
		}
		return nil
	}
	items := open("Yazar değeri")
	if has(items, "Özelliği düzenle…") == nil || has(items, "Tümünü seç") == nil {
		t.Fatalf("value's menu: %+v", items)
	}
	a.ctx.open = false
	has(items, "Yapıştır").run()
	tt.Advance(100 * time.Millisecond)
	if a.page.Props["yazar"] != "Herbert" {
		t.Fatalf("pasted: %q", a.page.Props["yazar"])
	}
	if items := open("Özellik: Yazar"); has(items, "Özelliği sil") == nil {
		t.Fatalf("name's menu: %+v", items)
	}
	a.ctx.open = false
	a.openCollection(coll)
	tt.Advance(300 * time.Millisecond)
	if items := open("Ara"); has(items, "Yapıştır") == nil {
		t.Fatalf("search's menu: %+v", items)
	}
}

// A right click in the page's text opens the app's menu: its text's lines and the block's; Kopyala
// copies the selection as Markdown.
func TestEditorOpensTheAppMenu(t *testing.T) {
	a, tt := newTestApp(t, "Birinci **kalın** satır\n")
	tt.SetSize(1100, 700)
	tt.Advance(300 * time.Millisecond)
	var copied string
	a.clipText = func(s string) { copied = s }
	a.doc.SetSelection(0, a.doc.Text().Len())
	tt.Frame()
	r := a.editorR
	tt.Move(r.X+60, r.Y+12)
	tt.Frame()
	tt.RightClickAt(r.X+60, r.Y+12)
	tt.Frame()
	if !a.ctx.open || tt.Menu() != nil {
		t.Fatalf("app menu %v, system menu %v", a.ctx.open, tt.Menu() != nil)
	}
	titles := map[string]func(){}
	for _, it := range a.ctx.stack[0] {
		titles[it.title] = it.run
	}
	for _, want := range []string{"Kopyala", "Kes", "Yapıştır", "Kalın", "Dönüştür…", "Sil"} {
		if titles[want] == nil {
			t.Fatalf("no %q in %v", want, titles)
		}
	}
	a.ctx.open = false
	titles["Kopyala"]()
	if copied != "Birinci **kalın** satır" {
		t.Fatalf("copied %q", copied)
	}
}

// The options' field: typed with commas they are chips, ⌫ in the empty field takes the last; its
// right click is the app's menu.
func TestTagsFieldAndItsMenu(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1100, 700)
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Özellik ekle"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(400 * time.Millisecond)
	tt.Type("Durum")
	if err := tt.Click("Seçenek ekle"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Açık,Kapalı,Bitti,")
	tt.Frame()
	if a.prop.options != "Açık,Kapalı,Bitti" {
		t.Fatalf("options %q", a.prop.options)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	tt.Frame()
	if a.prop.options != "Açık,Kapalı" {
		t.Fatalf("after ⌫: %q", a.prop.options)
	}
	r, _ := tt.Find("Seçenek ekle")
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !a.ctx.open || tt.Menu() != nil {
		t.Fatalf("app menu %v, system menu %v", a.ctx.open, tt.Menu() != nil)
	}
}
