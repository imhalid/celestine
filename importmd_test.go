package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestImportMarkdown: a vault's folders become collections, its files pages,
// with titles, tags and pictures carried over.
func TestImportMarkdown(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	vault := filepath.Join(t.TempDir(), "kasa")
	writeFile(t, filepath.Join(vault, "Kök not.md"), "# Kökteki\n\nmetin [[Başka]]\n")
	writeFile(t, filepath.Join(vault, "Projeler", "Plan.md"), "---\ntitle: Büyük Plan\ntags: [fikir, iş/yeni]\ncreated: 2025-03-04\n---\n\nbir ![[sema.png]] ve ![alt](img/foto.png) ve ![](yok.png)\n\n```\n#kodda\n```\n")
	writeFile(t, filepath.Join(vault, "Projeler", "Derin", "Alt.md"), "içerik\n")
	writeFile(t, filepath.Join(vault, "Projeler", "img", "foto.png"), string(pngBytes(8, 8)))
	writeFile(t, filepath.Join(vault, "Ekler", "sema.png"), string(pngBytes(9, 9)))
	writeFile(t, filepath.Join(vault, ".obsidian", "x.md"), "gizli\n")
	before := len(a.lib.Collections)

	st, err := a.importMarkdown(vault)
	if err != nil {
		t.Fatal(err)
	}
	if st.Collections != 2 || st.Pages != 3 || st.Pictures != 2 {
		t.Fatalf("stats %+v", st)
	}
	if len(a.lib.Collections) != before+2 {
		t.Fatalf("collections %d", len(a.lib.Collections))
	}
	var projeler, kok *Collection
	for _, c := range a.lib.Collections {
		switch c.Name {
		case "Projeler":
			projeler = c
		case "kasa":
			kok = c
		}
	}
	if projeler == nil || kok == nil || len(projeler.Pages) != 2 || len(kok.Pages) != 1 {
		t.Fatalf("shapes: %v %v", projeler, kok)
	}
	if kok.Pages[0].Title != "Kökteki" {
		t.Fatalf("title %q", kok.Pages[0].Title)
	}
	var plan *Page
	for _, p := range projeler.Pages {
		if p.Title == "Büyük Plan" {
			plan = p
		}
	}
	if plan == nil {
		t.Fatal("the front matter's title")
	}
	body, _ := plan.Text()
	if !strings.Contains(body, "#fikir #iş/yeni") || !strings.Contains(body, "![sema](resim-") || !strings.Contains(body, "![alt](resim-") || !strings.Contains(body, "![](yok.png)") {
		t.Fatalf("body:\n%s", body)
	}
	if plan.Created.Format("2006-01-02") != "2025-03-04" {
		t.Fatalf("created %v", plan.Created)
	}
	imgs, _ := filepath.Glob(filepath.Join(plan.Dir, "resim-*"))
	if len(imgs) != 2 {
		t.Fatalf("pictures: %v", imgs)
	}
	// the tags and the pictures work as the app's own
	a.linksStale()
	if got := tagsOf(body); !equal(got, []string{"fikir", "iş/yeni"}) {
		t.Fatalf("tags %v", got)
	}
	if _, err := a.importMarkdown(t.TempDir()); err == nil {
		t.Fatal("an empty folder")
	}
}

// A picture named by ../ out of the vault is not brought in; one named only by its name is found, as before,
// anywhere in the vault.
func TestImportPicturesStayInVault(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	base := t.TempDir()
	vault := filepath.Join(base, "kasa")
	writeFile(t, filepath.Join(base, "disarida.png"), string(pngBytes(7, 7))) // beside the vault, not in it
	writeFile(t, filepath.Join(vault, "Notlar", "A.md"), "![](../../disarida.png) ve ![[ic.png]] ve ![](../Ekler/ic2.png)\n")
	writeFile(t, filepath.Join(vault, "Ekler", "ic.png"), string(pngBytes(8, 8)))
	writeFile(t, filepath.Join(vault, "Ekler", "ic2.png"), string(pngBytes(9, 9)))
	st, err := a.importMarkdown(vault)
	if err != nil {
		t.Fatal(err)
	}
	if st.Pictures != 2 {
		t.Fatalf("pictures %d, want the two inside the vault", st.Pictures)
	}
}

// Names from outside that leave their folder are not names.
func TestSafeName(t *testing.T) {
	for n, ok := range map[string]bool{
		"notlar/a/page.json": true, "a": true, "a..b/c": true, ".cop/x/page.json": true,
		"": false, "../x": false, "a/../../x": false, "/etc/passwd": false, `..\x`: false, "a/b/..": false, "a\x00b": false,
	} {
		if SafeName(n) != ok {
			t.Errorf("safeName(%q) = %v", n, !ok)
		}
	}
	if _, ok := SafeSubpath("/kok", "../x"); ok {
		t.Error("a path out of the root")
	}
	if !Within("/kok", "/kok/a/b") || Within("/kok", "/kok/../x") || Within("/kok", "/kokos") {
		t.Error("within")
	}
}

// A note in the trash that says its page came from outside the library is not followed.
func TestRestoreUnsafeNote(t *testing.T) {
	lib := &Library{Root: t.TempDir(), FS: OSFS{}}
	for _, from := range []string{"../../x", "/etc/x", "notlar/../../x"} {
		if _, _, err := lib.Restore(&TrashItem{Kind: "page", Title: "x", From: from, Dir: t.TempDir()}); err == nil {
			t.Errorf("restored from %q", from)
		}
	}
}

// "Export as Markdown" is Markdown: a folder for a collection, a .md for a page with its front matter, the
// sub-pages in a folder beside it, the pictures beside the file; and what celestine imports it reads back.
func TestExportMarkdownRoundTrip(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	c := a.lib.Collections[0]
	p, err := c.NewPage(nil, "Dune: bölüm/bir")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveText("# Başlık\n\nmetin ![resim](foto.png)\n\n#etiket\n"); err != nil {
		t.Fatal(err)
	}
	if err := p.FS().WriteFile(filepath.Join(p.Dir, "foto.png"), pngBytes(8, 8)); err != nil {
		t.Fatal(err)
	}
	sub, err := c.NewPage(p, "Alt sayfa")
	if err != nil {
		t.Fatal(err)
	}
	sub.SaveText("alt metin\n")
	dir := filepath.Join(t.TempDir(), "md")
	if err := a.lib.ExportMarkdown(dir); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, FileSafe(c.Name))
	md, err := os.ReadFile(filepath.Join(folder, "Dune- bölüm-bir.md"))
	if err != nil {
		t.Fatalf("the page is not a .md: %v", err)
	}
	if !strings.HasPrefix(string(md), "---\n") || !strings.Contains(string(md), "title: ") || !strings.Contains(string(md), "metin") {
		t.Fatalf("the file: %q", md)
	}
	if _, err := os.Stat(filepath.Join(folder, "foto.png")); err != nil {
		t.Fatalf("the picture is not beside it: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(folder, "Dune- bölüm-bir", "Alt sayfa.md")); err != nil || !strings.Contains(string(b), "alt metin") {
		t.Fatalf("the sub-page: %v %q", err, b)
	}
	// no page.json, no trash
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if strings.HasSuffix(path, "page.json") || strings.Contains(path, ".cop") {
			t.Errorf("a library file in the export: %s", path)
		}
		return nil
	})
	b, _ := newTestApp(t, "y\n")
	st, err := b.importMarkdown(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Pages != 3 || st.Pictures != 1 { // the library's own first page, the page and its sub-page
		t.Fatalf("read back: %+v", st)
	}
}

// What the quick panel draws each frame asks where the note goes, and makes nothing to answer.
func TestQuickWhereMakesNothing(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	before := len(a.lib.Collections)
	q := &quickNote{a: a}
	q.where()
	q.mode = quickPage
	q.where()
	if len(a.lib.Collections) != before {
		t.Fatalf("collections %d -> %d", before, len(a.lib.Collections))
	}
}
