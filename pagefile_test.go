package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPageFileRoundTrip(t *testing.T) {
	meta := map[string]string{"title": "Dune", "order": "2"}
	src := JoinPage(meta, encodeBody(importMarkdown("# Başlık\n\nbir **kalın** [[Sayfa]] #etiket\n\n- [ ] görev\n")))
	m, body := SplitPage(src)
	if m["title"] != "Dune" || m["order"] != "2" || PageMeta(src)["title"] != "Dune" {
		t.Fatalf("meta %v", m)
	}
	if got := bodyText(body); got != "# Başlık\n\nbir **kalın** [[Sayfa]] #etiket\n\n- [ ] görev" {
		t.Fatalf("text %q", got)
	}
	if again := encodeBody(decodeBody(body)); again != body {
		t.Fatalf("not stable:\n%s\n%s", body, again)
	}
	if m, body := SplitPage([]byte("bozuk")); len(m) != 0 || body != "[]" {
		t.Fatalf("a file that is not one: %v %q", m, body)
	}
}

// A page folder of the time of Markdown files is made a page file once.
func TestLegacyPageMigrates(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "k")
	os.MkdirAll(filepath.Join(dir, "eski"), 0o755)
	os.WriteFile(filepath.Join(dir, CollectionFile), []byte(`{"name":"K","subPages":true}`), 0o644)
	os.WriteFile(filepath.Join(dir, "eski", LegacyPageFile), []byte("---\ntitle: Eski\norder: 1\n---\n\n# Eski\n\nmetin [[Başka]]\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"), 0o644)
	lib, err := OpenLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	p := lib.Collections[0].Pages[0]
	text, _ := p.Text()
	if p.Title != "Eski" || !strings.Contains(text, "metin [[Başka]]") || strings.Contains(text, "# Eski") || !strings.Contains(text, tableLang) {
		t.Fatalf("%q %q", p.Title, text)
	}
	if _, err := os.Stat(filepath.Join(dir, "eski", LegacyPageFile)); err == nil {
		t.Fatal("the old file stays")
	}
}
