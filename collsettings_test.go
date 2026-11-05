package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestCollSettings opens a collection's settings, changes it,
// cancels (as it was), changes again and saves (kept, on disk).
func TestCollSettings(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	if err := tt.Click("Koleksiyon ayarları"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(600 * time.Millisecond)
	if !a.cs.open || a.cs.coll != coll {
		t.Fatal("not open")
	}
	a.cs.draft.Name, a.cs.draft.Icon, a.cs.draft.Color = "Okuma", "book", "c3"
	tt.Click("Vazgeç")
	tt.Advance(time.Second)
	if coll.Name != "Kitaplar" || coll.Icon != "" || a.cs.open {
		t.Fatalf("cancel: %q %q", coll.Name, coll.Icon)
	}
	// again: a property, a template, a page noun, saved
	a.openCollSettings(coll, 1)
	tt.Frame() // opened from outside a frame: ask for one
	tt.Advance(1500 * time.Millisecond)
	a.cs.newName, a.cs.newType, a.cs.newOptions = "Durum", 0, "Okunacak, Okundu"
	a.addDraftProp()
	a.cs.draft.Template = "## Notlar\n"
	a.cs.draft.PageNoun = "kitap"
	a.cs.draft.Description = "Okuduklarım"
	tt.Frame()
	if coll.Prop("durum") != nil {
		t.Fatal("the draft changed the collection before saving")
	}
	if err := tt.Click("Kaydet"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if coll.Prop("durum") == nil || coll.Template != "## Notlar\n" || !tt.HasText("Okuduklarım") || !tt.HasText("Yeni kitap") {
		t.Fatalf("saved: %+v %v", coll.Properties, tt.Texts())
	}
	data, _ := os.ReadFile(coll.Dir + "/" + collectionFile)
	for _, want := range []string{`"pageNoun": "kitap"`, `"template": "## Notlar\n"`, `"name": "Durum"`, `"type": "board"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("not on disk: %s", want)
		}
	}
	// a new page starts with the template
	tt.Click("Yeni kitap")
	tt.Frame()
	if body, _ := a.page.Body(); body != "## Notlar\n" {
		t.Fatalf("template: %q", body)
	}
}

// TestSharedProps: a property of another collection is offered by name.
func TestSharedProps(t *testing.T) {
	lib, _, one, two := dragLibrary(t)
	one.AddProperty("Yazar", PropText, nil)
	one.AddProperty("Durum", PropChoice, []string{"Açık", "Kapalı"})
	a := newApp(lib)
	a.openCollSettings(two, 1)
	if got := a.sharedProps("dur"); len(got) != 1 || got[0].p.Name != "Durum" || got[0].from != "Bir" {
		t.Fatalf("offered: %+v", got)
	}
	a.cs.draft.Properties = append(a.cs.draft.Properties, got(a, "Durum"))
	if got := a.sharedProps("dur"); len(got) != 0 {
		t.Fatal("offered again once in the draft")
	}
}

func got(a *app, name string) Property { return a.sharedProps(name)[0].p }

// TestViewOptions: a table shows the view's columns; a board's column folds.
func TestViewOptions(t *testing.T) {
	_, tt, coll := viewsLibrary(t)
	coll.AddProperty("Durum", PropChoice, []string{"Yapılacak", "Bitti"})
	coll.AddProperty("Yazar", PropText, nil)
	for i := range coll.Views {
		if coll.Views[i].Type == "table" {
			coll.Views[i].Columns = []string{"prop:yazar", "created"}
		}
	}
	tt.Click("Tablo")
	tt.Advance(300 * time.Millisecond)
	if !tt.HasText("Yazar") || tt.HasText("Üst sayfa") || tt.HasText("Değiştirilme") {
		t.Fatalf("columns: %v", tt.Texts())
	}
	tt.Click("Pano")
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click("Daralt: Bitti"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	v := coll.View()
	if len(v.Collapsed) != 1 || v.Collapsed[0] != "Bitti" {
		t.Fatalf("folded: %+v", v.Collapsed)
	}
	if err := tt.Click("Aç: Bitti"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(coll.View().Collapsed) != 0 {
		t.Fatal("not unfolded")
	}
}
