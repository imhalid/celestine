package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/imhalid/celestine/locales"
)

// The page, tags and file properties on a page and in a list: a page link shows its page's title and opens it; tags
// are chips; a file is copied into the page's folder, a second of the same name numbered, the one replaced and the
// one removed gone from the folder.
func TestPropertyKinds(t *testing.T) {
	a, tt, coll := viewsLibrary(t)
	link, _ := coll.AddProperty("Benzeri", PropPage, nil)
	tags, _ := coll.AddProperty("Etiket", PropTags, nil)
	file, _ := coll.AddProperty("Ek", PropFile, nil)
	byTitle := map[string]*Page{}
	for _, p := range coll.All() {
		byTitle[p.Title] = p
	}
	dune, anna := byTitle["Dune"], byTitle["Anna Karenina"]
	if err := dune.SetValue(link.ID, anna.Path()); err != nil {
		t.Fatal(err)
	}
	if err := dune.SetValue(tags.ID, "bilimkurgu, klasik"); err != nil {
		t.Fatal(err)
	}
	coll.View().Fields = []string{link.ID, tags.ID}
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	for _, want := range []string{"bilimkurgu", "klasik"} {
		if !tt.HasText(want) {
			t.Fatalf("the list does not show %q: %v", want, tt.Texts())
		}
	}
	a.open(dune)
	tt.Advance(400 * time.Millisecond)
	if !tt.HasText("Anna Karenina") || !tt.HasText("klasik") {
		t.Fatalf("the page's properties: %v", tt.Texts())
	}
	if err := tt.Click("Benzeri değeri"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(300 * time.Millisecond)
	if err := tt.Click(locales.PropsOpenPage("Anna Karenina")); err != nil {
		t.Fatal(err, tt.Texts())
	}
	tt.Frame()
	if a.page != anna {
		t.Fatalf("the link did not open its page: %v", a.page)
	}
	// files
	src := filepath.Join(t.TempDir(), "plan.pdf")
	os.WriteFile(src, []byte("%PDF one"), 0o644)
	a.setPropFile(dune, file, src)
	if dune.Props[file.ID] != "plan.pdf" {
		t.Fatalf("value %q", dune.Props[file.ID])
	}
	if b, _ := os.ReadFile(filepath.Join(dune.Dir, "plan.pdf")); string(b) != "%PDF one" {
		t.Fatal("the file is not in the page's folder")
	}
	os.WriteFile(filepath.Join(dune.Dir, "taken.pdf"), []byte("x"), 0o644)
	other := filepath.Join(t.TempDir(), "taken.pdf")
	os.WriteFile(other, []byte("%PDF two"), 0o644)
	a.setPropFile(dune, file, other)
	if dune.Props[file.ID] != "taken (2).pdf" {
		t.Fatalf("a name taken: %q", dune.Props[file.ID])
	}
	if _, err := os.Stat(filepath.Join(dune.Dir, "plan.pdf")); !os.IsNotExist(err) {
		t.Fatal("the replaced file stays")
	}
	a.setPropFile(dune, file, "")
	if _, err := os.Stat(filepath.Join(dune.Dir, "taken (2).pdf")); !os.IsNotExist(err) || dune.Props[file.ID] != "" {
		t.Fatal("the removed file stays")
	}
}

func TestSetPropDataRejectsTraversalAndPreservesUnsafeOld(t *testing.T) {
	a, _, coll := viewsLibrary(t)
	fileProp, _ := coll.AddProperty("Attach", PropFile, nil)
	p := coll.All()[0]
	outside := filepath.Join(filepath.Dir(filepath.Dir(p.Dir)), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Simulate an untrusted page.json loaded from disk/sync carrying an unsafe PropFile value.
	unsafeOld := filepath.Join("..", "..", "outside.txt")
	if p.Props == nil {
		p.Props = map[string]string{}
	}
	p.Props[fileProp.ID] = unsafeOld
	for _, bad := range []string{"../evil.txt", `..\evil.txt`, "/tmp/evil.txt", "sub/evil.txt", ".", ".."} {
		a.toasts.items = nil
		a.setPropData(p, fileProp, bad, []byte("pwn"))
		if len(a.toasts.items) == 0 || a.toasts.items[0].text != locales.CommonInvalidAttachmentName(bad) {
			t.Fatalf("bad name %q error toast = %+v, want %q", bad, a.toasts.items, locales.CommonInvalidAttachmentName(bad))
		}
	}
	if got := p.Props[fileProp.ID]; got != unsafeOld {
		t.Fatalf("unsafe base mutated property to %q", got)
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "keep" {
		t.Fatalf("outside file modified: %v %q", err, string(b))
	}
	// Replacing with a valid file must NOT delete an unsafe old path.
	a.setPropData(p, fileProp, "good.txt", []byte("ok"))
	if p.Props[fileProp.ID] != "good.txt" {
		t.Fatalf("valid file not saved: %q", p.Props[fileProp.ID])
	}
	if b, err := os.ReadFile(outside); err != nil || string(b) != "keep" {
		t.Fatalf("replacing removed outside target: %v %q", err, string(b))
	}
}
