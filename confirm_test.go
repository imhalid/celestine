package main

import (
	"testing"
)

// What cannot be taken back asks first, and what changes many settings or a collection's schema can be undone
// (#217).
func TestAskAndUndo(t *testing.T) {
	a, _ := newTestApp(t, "x\n")
	defer setLang("tr") // undoing a reset sets the language the settings had: the tests' own afterwards
	p := a.pane().page
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

	p, _ = a.lib.Collections[0].NewPage(nil, "Özellikli")
	if _, err := p.AddProperty("Durum", "text", nil); err != nil {
		t.Fatal(err)
	}
	p.SetValue("durum", "açık")
	a.removePageProperty(p, "durum")
	if p.Prop("durum") != nil || p.Props["durum"] != "" {
		t.Fatal("not removed")
	}
	undoLast(t, a)
	if p.Prop("durum") == nil || p.Props["durum"] != "açık" {
		t.Fatal("the property was not brought back with its value")
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
