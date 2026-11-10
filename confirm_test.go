package main

import (
	"testing"
)

// What cannot be taken back asks first, and what changes many settings or a collection's schema can be undone
// (#217).
func TestAskAndUndo(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	p := a.page
	if _, err := a.lib.TrashPage(p); err != nil {
		t.Fatal(err)
	}
	a.loadTrash()
	a.deleteForever(a.trashItems[0])
	if len(a.lib.Trash()) != 1 || !a.asking.open {
		t.Fatal("deleted without asking")
	}
	a.asking.run()
	if len(a.lib.Trash()) != 0 {
		t.Fatal("not deleted when it was asked for")
	}

	a.settings.ShowSpaces, a.settings.Theme = true, 2
	before := a.settings
	a.resetAllAsked()
	if !a.asking.open || a.settings.Theme != 2 {
		t.Fatal("reset without asking")
	}
	a.asking.run()
	if a.settings.Theme != 0 || a.settings.ShowSpaces {
		t.Fatal("not reset")
	}
	undoLast(t, a)
	if a.settings.Theme != before.Theme || !a.settings.ShowSpaces {
		t.Fatal("the reset was not undone")
	}

	coll := a.lib.Collections[0]
	if _, err := coll.AddProperty("Durum", "text", nil); err != nil {
		t.Fatal(err)
	}
	a.removeProperty(coll, "durum")
	if coll.Prop("durum") != nil {
		t.Fatal("not removed")
	}
	undoLast(t, a)
	if coll.Prop("durum") == nil {
		t.Fatal("the property was not brought back")
	}
}

// undoLast presses the button of the last toast.
func undoLast(t *testing.T, a *app) {
	t.Helper()
	items := a.toasts.items
	if len(items) == 0 || items[len(items)-1].fn == nil {
		t.Fatal("no toast to undo with")
	}
	items[len(items)-1].fn()
}
