package main

import "github.com/egoist/mygo"

// The menu bar, where a Mac user looks first: every command with its
// shortcut. In the app the menu bar has the shortcuts (a.menuBar), so the
// window doesn't take them twice; tests, without one, press them on the
// window.

func (a *app) menu() *mygo.Menu {
	run := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { a.do(func() error { fn(); return nil }) }
	}
	item := func(label, acc string, fn func()) *mygo.MenuItem {
		return &mygo.MenuItem{Label: label, Accelerator: acc, Click: run(fn)}
	}
	newPage := func() {
		switch {
		case a.coll != nil:
			a.newPage(a.coll, nil)
		case a.page != nil:
			a.newPage(a.page.Coll, nil)
		case len(a.lib.Collections) > 0:
			a.newPage(a.lib.Collections[0], nil)
		}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "ryudit", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			item("Ayarlar…", "CmdOrCtrl+,", func() { a.settingsOpen = true }),
			mygo.Separator(),
			{Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Label: "Dosya", Submenu: []*mygo.MenuItem{
			item("Yeni sayfa", "CmdOrCtrl+N", newPage),
			item("Yeni koleksiyon", "CmdOrCtrl+Shift+N", func() {
				if c, err := a.lib.NewCollection("Yeni koleksiyon"); err == nil {
					a.newPage(c, nil)
				} else {
					a.saveErr = err.Error()
				}
			}),
			mygo.Separator(),
			item("Çöp", "", a.openTrash),
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "Görünüm", Submenu: []*mygo.MenuItem{
			item("Kenar çubuğu", "CmdOrCtrl+\\", a.toggleSidebar),
			item("Caret'i değiştir", "CmdOrCtrl+Shift+M", func() { a.caretKind = (a.caretKind + 1) % 2 }),
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "Git", Submenu: []*mygo.MenuItem{
			item("Geri", "CmdOrCtrl+[", func() { a.step(-1) }),
			item("İleri", "CmdOrCtrl+]", func() { a.step(1) }),
		}},
		{Role: mygo.RoleWindowMenu},
	})
}
