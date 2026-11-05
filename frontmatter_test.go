package main

import (
	"os"
	"strings"
	"testing"
)

// TestFrontMatterKept: a page with front matter from another tool - lists,
// nested values, comments, keys we don't know - keeps all of it when we
// save it, with only our keys changed.
func TestFrontMatterKept(t *testing.T) {
	lib, ps, one, _ := dragLibrary(t)
	one.AddProperty("Durum", PropChoice, []string{"Açık", "Kapalı"})
	src := `---
title: A
# Obsidian'ın yazdığı
tags:
  - okuma
  - "bilim kurgu"
aliases: [Dune, Arrakis]
cssclasses: geniş
yazar:
  ad: Frank
  soyad: Herbert
published: 1965
draft: false
---

Metin.
`
	os.WriteFile(ps["A"].file(), []byte(src), 0o644)
	again, _ := OpenLibrary(lib.Root)
	var a *Page
	for _, c := range again.Collections {
		for _, p := range c.All() {
			if p.Title == "A" {
				a = p
			}
		}
	}
	if a.Props["tags"] != "okuma, bilim kurgu" || a.Props["aliases"] != "Dune, Arrakis" || a.Props["published"] != "1965" {
		t.Fatalf("read: %+v", a.Props)
	}
	if err := a.SetTitle("A (1965)"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetValue("durum", "Kapalı"); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(a.file())
	got := string(out)
	for _, want := range []string{
		"title: A (1965)\n",
		"# Obsidian'ın yazdığı\n",
		"tags:\n  - okuma\n  - \"bilim kurgu\"\n",
		"aliases: [Dune, Arrakis]\n",
		"yazar:\n  ad: Frank\n  soyad: Herbert\n",
		"published: 1965\n",
		"draft: false\n",
		"durum: Kapalı\n",
		"---\n\nMetin.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q in:\n%s", want, got)
		}
	}
	if strings.Index(got, "tags:") > strings.Index(got, "durum:") {
		t.Errorf("the order of keys changed:\n%s", got)
	}
}

// TestFrontMatterBad: front matter that is not YAML is kept as it is; the
// text is still saved, and the error says why the title was not.
func TestFrontMatterBad(t *testing.T) {
	lib, ps, _, _ := dragLibrary(t)
	src := "---\ntitle: A\nbozuk: [kapanmayan\n---\n\nMetin.\n"
	os.WriteFile(ps["A"].file(), []byte(src), 0o644)
	again, _ := OpenLibrary(lib.Root)
	var a *Page
	for _, c := range again.Collections {
		for _, p := range c.All() {
			if strings.HasPrefix(p.Title, "A") || p.Dir == ps["A"].Dir {
				a = p
			}
		}
	}
	if err := a.Save("Yeni metin.\n"); err != errBadFront {
		t.Fatalf("err %v", err)
	}
	out, _ := os.ReadFile(a.file())
	if string(out) != "---\ntitle: A\nbozuk: [kapanmayan\n---\n\nYeni metin.\n" {
		t.Fatalf("written: %q", out)
	}
}

// TestFrontMatterQuotes: values YAML would read as something else are quoted.
func TestFrontMatterQuotes(t *testing.T) {
	f := &front{}
	f.set("title", "yes")
	f.set("a", "Başlık: alt")
	f.set("b", "düz")
	if got := f.text(); got != "title: \"yes\"\na: \"Başlık: alt\"\nb: düz\n" {
		t.Fatalf("%q", got)
	}
	g, body := readFront("---\n" + f.text() + "---\n\nx\n")
	if g.get("title") != "yes" || g.get("a") != "Başlık: alt" || body != "x\n" {
		t.Fatalf("read back: %q %q %q", g.get("title"), g.get("a"), body)
	}
}
