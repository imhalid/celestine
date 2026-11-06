package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Tab in plain text writes a tab where the caret is; ⇧Tab takes it back.
func TestTabInPlainText(t *testing.T) {
	a, tt := newTestApp(t, "Önce\n")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyDown)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if got := a.doc.Text().String(); got != "Önce\t" {
		t.Fatalf("after Tab: %q", got)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()
	if got := a.doc.Text().String(); got != "Önce" {
		t.Fatalf("after ⇧Tab: %q", got)
	}
}

// Tab is the text's in a list (a level deeper), in a block of code (a tab) and in a heading,
// and never moves the focus out of the text.
func TestTabInEveryParagraph(t *testing.T) {
	for _, c := range []struct {
		md   string
		want func(a *app) bool
	}{
		{"- bir\n", func(a *app) bool { _, c := a.doc.Selection(); return a.doc.Text().ParagraphStyleAt(c).Indent == 2 }},
		{"```\nx\n```\n", func(a *app) bool { return a.doc.Text().String() != "" && containsTab(a.doc.Text().String()) }},
		{"# Baş\n", func(a *app) bool { return containsTab(a.doc.Text().String()) }},
	} {
		a, tt := newTestApp(t, c.md)
		tt.Frame()
		tt.Key(ui.Cmd, ui.KeyDown)
		tt.Frame()
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if !a.focused {
			t.Errorf("%q: the text lost the focus", c.md)
		}
		if !c.want(a) {
			t.Errorf("%q: after Tab: %q", c.md, a.doc.Text().String())
		}
	}
}

func containsTab(s string) bool {
	for _, r := range s {
		if r == '\t' {
			return true
		}
	}
	return false
}
