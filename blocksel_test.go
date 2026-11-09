package main

import (
	"testing"
	"time"
)

// Blocks chosen from the keyboard: V in normal mode chooses the caret's block, j takes in the next; y copies them as
// Markdown; J moves them down past the block after them, K back; d deletes them; Esc lets them go.
func TestBlockSelection(t *testing.T) {
	k := newKbd(t, "one\n\ntwo\n\nthree\n\nfour\n")
	copied := ""
	k.a.clipText = func(s string) { copied = s }
	k.a.doc.SetSelection(0, 0)
	k.frames()
	k.press("esc", "V", "j")
	if !k.a.bsel.on {
		t.Fatal("V chose nothing")
	}
	if s, e := k.a.blockSelRange(); k.a.doc.Text().Slice(s, e).String() != "one\ntwo" {
		t.Fatalf("chosen: %q", k.a.doc.Text().Slice(s, e).String())
	}
	k.press("y")
	if copied != "one\n\ntwo" || k.a.bsel.on {
		t.Fatalf("y copied %q, still chosen %v", copied, k.a.bsel.on)
	}
	k.press("V", "j", "J")
	if got := k.a.doc.Text().String(); got != "three\none\ntwo\nfour" {
		t.Fatalf("after J: %q", got)
	}
	if s, e := k.a.blockSelRange(); k.a.doc.Text().Slice(s, e).String() != "one\ntwo" {
		t.Fatalf("not chosen where they went: %q", k.a.doc.Text().Slice(s, e).String())
	}
	k.press("K")
	if got := k.a.doc.Text().String(); got != "one\ntwo\nthree\nfour" {
		t.Fatalf("after K: %q", got)
	}
	k.press("d")
	if got := k.a.doc.Text().String(); got != "three\nfour" || k.a.bsel.on {
		t.Fatalf("after d: %q", got)
	}
	k.press("V", "esc")
	if k.a.bsel.on {
		t.Fatal("Esc did not let the blocks go")
	}
}

// The block menu opens from the keyboard for the caret's block; a block opens beside, in the next pane, there.
func TestBlockMenuAndBeside(t *testing.T) {
	a, tt := newTestApp(t, "one\n\ntwo\n\nthree\n")
	tt.Advance(100 * time.Millisecond)
	a.doc.SetSelection(5, 5) // in "two"
	a.openBlockMenu()
	tt.Advance(200 * time.Millisecond)
	if !a.grip.menu || a.grip.para != 4 {
		t.Fatalf("the menu of the caret's block: open %v, at %d", a.grip.menu, a.grip.para)
	}
	a.grip.menu = false
	a.openBlockBeside()
	tt.Advance(300 * time.Millisecond)
	if a.paneCount() != 2 {
		t.Fatalf("no pane beside: %d", a.paneCount())
	}
	_, at := a.doc.Selection()
	if at != 4 {
		t.Fatalf("the pane beside is not at the block: %d", at)
	}
}
