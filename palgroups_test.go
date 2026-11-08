package main

import (
	"fmt"
	"strings"
	"testing"
)

// With nothing typed, the commands are in groups, and a family of many (a move to each of 40
// collections) is one line, which opens to them all; typed, each is found by itself.
func TestPaletteGroupsCommands(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	for i := range 40 {
		if _, err := a.lib.NewCollection(fmt.Sprintf("Küçük %03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	tt.Frame()
	items := a.commandItems("")
	moves, fam := 0, -1
	groups := []string{}
	for i, it := range items {
		if strings.HasPrefix(it.title, "Sayfayı taşı: ") {
			moves++
		}
		if it.title == "Sayfayı taşı…" {
			fam = i
		}
		if len(groups) == 0 || groups[len(groups)-1] != it.group {
			groups = append(groups, it.group)
		}
	}
	if moves != 0 || fam < 0 || items[fam].keys != "40 seçenek" {
		t.Fatalf("moves %d, family at %d: %+v", moves, fam, items[max(fam, 0)])
	}
	if len(items) > 30 {
		t.Fatalf("%d lines with nothing typed", len(items))
	}
	seen := map[string]bool{}
	for _, g := range groups { // each group once, in one run
		if seen[g] {
			t.Fatalf("group %q twice: %v", g, groups)
		}
		seen[g] = true
	}
	a.pal = palette{open: true, query: ">"}
	a.runItem(items[fam])
	if !a.pal.open || a.pal.query != ">Sayfayı taşı: " {
		t.Fatalf("the family did not open: %+v", a.pal)
	}
	if got := a.commandItems("Sayfayı taşı: "); len(got) < 40 {
		t.Fatalf("its members: %d", len(got))
	}
	if got := a.commandItems("küçük 017"); len(got) == 0 || got[0].title != "Sayfayı taşı: Küçük 017" {
		t.Fatalf("typed: %+v", got[:min(len(got), 3)])
	}
}

// A group's line opens the group: its commands, a family of them as one line, and the way back.
func TestPaletteOpensAGroup(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.Frame()
	a.pal = palette{open: true, query: ">"}
	var look palItem
	for _, it := range a.commandItems("") {
		if it.group == "Gruplar" && it.title == "Görünüm" {
			look = it
		}
	}
	if look.run == nil {
		t.Fatal("no Görünüm group")
	}
	a.runItem(look)
	if !a.pal.open || a.pal.cat != "Görünüm" {
		t.Fatalf("not opened: %+v", a.pal)
	}
	items := a.commandItems("")
	titles := map[string]bool{}
	for _, it := range items {
		titles[it.title] = true
		if it.group != "Görünüm" {
			t.Fatalf("%q under %q", it.title, it.group)
		}
	}
	if !titles["Bütün gruplar"] || !titles["Caret…"] || !titles["Renk teması…"] {
		t.Fatalf("lines: %v", titles)
	}
	a.runItem(items[0]) // back
	if a.pal.cat != "" || !a.pal.open {
		t.Fatalf("not back: %+v", a.pal)
	}
}
