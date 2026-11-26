package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
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

// A group's line opens the group: its commands and a family of them as one line, with no line to go back by (#603).
func TestPaletteOpensAGroup(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.Frame()
	a.pal = palette{open: true, query: ">"}
	var look palItem
	for _, it := range a.commandItems("") {
		if it.group == "groups" && it.title == msg("common_appearance") {
			look = it
		}
	}
	if look.run == nil {
		t.Fatal("no Görünüm group")
	}
	a.runItem(look)
	if !a.pal.open || a.pal.cat != "common_appearance" {
		t.Fatalf("not opened: %+v", a.pal)
	}
	items := a.commandItems("")
	titles := map[string]bool{}
	for _, it := range items {
		titles[it.title] = true
		if it.group != "common_appearance" {
			t.Fatalf("%q under %q", it.title, it.group)
		}
	}
	if titles["Bütün gruplar"] || !titles["Caret…"] || !titles["Renk teması…"] {
		t.Fatalf("lines: %v", titles)
	}
	a.palBack()
	if a.pal.cat != "" || !a.pal.open {
		t.Fatalf("not back: %+v", a.pal)
	}
}

// With nothing typed, Right opens the group or the family chosen and Left goes back out to it, chosen again;
// with words typed, the arrows move the caret and do not leave the list (#603).
func TestPaletteArrowsOpenAndLeave(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	a.openCommands()
	tt.Frame()
	items := a.palLines()
	g := -1
	for i, it := range items {
		if it.group == "groups" && it.title == msg("common_appearance") {
			g = i
		}
	}
	if g < 0 {
		t.Fatal("no Görünüm group")
	}
	a.pal.sel = g
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.pal.cat != "common_appearance" || !a.pal.open {
		t.Fatalf("Right did not open the group: %+v", a.pal.cat)
	}
	fam := -1
	for i, it := range a.palLines() {
		if it.drill {
			fam = i
		}
	}
	if fam < 0 {
		t.Fatal("no family in the group")
	}
	a.pal.sel = fam
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.pal.cat != "" || !strings.HasSuffix(a.pal.query, ": ") {
		t.Fatalf("Right did not open the family: %q", a.pal.query)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if a.pal.cat != "common_appearance" || a.pal.sel != fam {
		t.Fatalf("Left did not go back to the group: cat %q sel %d, want %d", a.pal.cat, a.pal.sel, fam)
	}
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if a.pal.cat != "" || a.pal.query != ">" || a.pal.sel != g || !a.pal.open {
		t.Fatalf("Left did not go back to the groups: cat %q query %q sel %d, want %d", a.pal.cat, a.pal.query, a.pal.sel, g)
	}
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if a.pal.cat != "" || a.pal.sel != g || !a.pal.open {
		t.Fatalf("⌫ did not go back to the groups: cat %q sel %d", a.pal.cat, a.pal.sel)
	}
	typeEach(tt, "tema")
	tt.Frame()
	tt.Key(0, ui.KeyLeft)
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if a.pal.query != ">tema" || a.pal.cat != "" || !a.pal.open {
		t.Fatalf("the arrows left the typed query: %q cat %q", a.pal.query, a.pal.cat)
	}
}

// With the pointer resting over the list, Right and Left choose what the keys choose: the list shown anew does not
// take the row under the pointer for one it moved to, and Left goes back to the group's row (#616).
func TestPaletteArrowsKeepTheChoice(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	a.openCommands()
	tt.Frame()
	g := -1
	for i, it := range a.palLines() {
		if it.group == "groups" && it.title == msg("common_appearance") {
			g = i
		}
	}
	if g < 0 {
		t.Fatal("no Görünüm group")
	}
	tt.Move(500, 260) // over the list, and resting there
	tt.Frame()
	a.pal.sel = g
	tt.Frame()
	tt.Key(0, ui.KeyRight)
	for range 4 {
		tt.Frame()
		if a.pal.cat != "common_appearance" || a.pal.sel != 0 {
			t.Fatalf("in the group: cat %q, row %d, want the first", a.pal.cat, a.pal.sel)
		}
	}
	tt.Key(0, ui.KeyLeft)
	for range 4 {
		tt.Frame()
		if a.pal.cat != "" || a.pal.sel != g {
			t.Fatalf("back: row %d, want the group's %d", a.pal.sel, g)
		}
	}
}

// Down to the list's last row shows it with the list's room under it, clear of the card's edge, and Up back to the
// first shows its group's heading over it: the rows reached with the keys need no scrolling with the pointer (#617).
func TestPaletteArrowsShowTheRoom(t *testing.T) {
	a, tt := newTestApp(t, "Metin\n")
	tt.SetSize(1000, 700)
	tt.Frame()
	a.openCommands()
	tt.Frame()
	items := a.palLines()
	for range len(items) {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Frame()
	card, ok := tt.Find(a.palLabel())
	last, ok2 := tt.Find(items[len(items)-1].title)
	if !ok || !ok2 {
		t.Fatal("no card or no last row")
	}
	if gap := card.Y + card.H - (last.Y + last.H); gap < 8 {
		t.Fatalf("the last row is %.1f from the card's bottom edge", gap)
	}
	for range len(items) {
		tt.Key(0, ui.KeyUp)
		tt.Frame()
	}
	tt.Frame()
	head, ok := tt.Find(msg(items[0].group))
	first, _ := tt.Find(items[0].title)
	if !ok || head.Y > first.Y || head.Y < card.Y {
		t.Fatalf("the first row's heading is not shown: %+v over %+v in %+v", head, first, card)
	}
}
