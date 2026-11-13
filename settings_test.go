package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
)

// TestLibraryShowInFolder: Settings ▸ Library's Show in folder shows the library's file in its folder (#485).
func TestLibraryShowInFolder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "k.db")
	st, err := OpenStore(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := OpenLibraryIn(st, StoreRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, err := lib.NewCollection("Notes")
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.NewPage(nil, "Home")
	if err != nil {
		t.Fatal(err)
	}
	a := newApp(lib)
	a.open(p)
	a.noSound = true
	type call struct {
		path   string
		reveal bool
	}
	opened := make(chan call, 4)
	a.shellOpen = func(path string, reveal bool) { opened <- call{path, reveal} }
	tt := ui.NewTester(a.view, 1100, 900)
	tt.Advance(50 * time.Millisecond)
	a.settingsOpen, a.settingsTab = true, tabLibrary
	tt.Advance(900 * time.Millisecond)
	if err := tt.Click(locales.PropsShowFolder()); err != nil {
		t.Fatalf("no Show in folder: %v", tt.Texts())
	}
	select {
	case got := <-opened:
		if got.path != file || !got.reveal {
			t.Fatalf("showed %+v, want the library's file %s", got, file)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Show in folder did nothing")
	}
}
