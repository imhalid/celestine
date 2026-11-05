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
	body, _ := plan.Body()
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
