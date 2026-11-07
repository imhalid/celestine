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
		return &mygo.MenuItem{Label: T(label), Accelerator: acc, Click: run(fn)}
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
			item("Ayarlar…", "CmdOrCtrl+,", a.toggleSettings),
			mygo.Separator(),
			{Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Label: T("Dosya"), Submenu: []*mygo.MenuItem{
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
			item("Kaydet", "CmdOrCtrl+S", a.saveNow),
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: T("Görünüm"), Submenu: []*mygo.MenuItem{
			item("Kenar çubuğu", "CmdOrCtrl+\\", a.toggleSidebar),
			item("Yan panel", "CmdOrCtrl+Shift+\\", a.togglePanel),
			item("Caret'i değiştir", "CmdOrCtrl+Shift+M", func() { a.setCaret((a.caretKind + 1) % caretCount()) }),
			item("Yazıyı büyüt", "CmdOrCtrl+=", func() { a.stepFont(1) }),
			item("Yazıyı küçült", "CmdOrCtrl+-", func() { a.stepFont(-1) }),
			item("Yazı boyutu: varsayılan", "CmdOrCtrl+0", a.resetFont),
			mygo.Separator(),
			item("Yan bölmede aç (bu sayfa)", "CmdOrCtrl+Alt+Enter", a.openHereInSide),
			item("Bölmeler arası geç", "CmdOrCtrl+Alt+Right", a.nextPane),
			item("Bölmeyi kapat", "CmdOrCtrl+Alt+W", a.closeThisPane),
			mygo.Separator(),
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: T("Blok"), Submenu: []*mygo.MenuItem{
			item("Yukarı taşı", "CmdOrCtrl+Shift+Up", func() {
				if a.doc != nil {
					moveBlock(a.doc, -1)
				}
			}),
			item("Aşağı taşı", "CmdOrCtrl+Shift+Down", func() {
				if a.doc != nil {
					moveBlock(a.doc, 1)
				}
			}),
			item("Çoğalt", "CmdOrCtrl+D", func() {
				if a.doc != nil {
					_, caret := a.doc.Selection()
					duplicateBlock(a.doc, caret)
				}
			}),
		}},
		{Label: T("Git"), Submenu: []*mygo.MenuItem{
			item("Sayfa ara…", "CmdOrCtrl+K", a.openPalette),
			item("Komutlar…", "CmdOrCtrl+P", a.openCommands),
			item("Komutlar… (⇧⌘P)", "CmdOrCtrl+Shift+P", a.openCommands),
			item("Sayfada bul…", "CmdOrCtrl+F", a.openFind),
			item("Kitaplıkta ara…", "CmdOrCtrl+Shift+F", func() { a.openSearch(a.searchQ) }),
			mygo.Separator(),
			item("Bugünün sayfası", "CmdOrCtrl+J", a.openToday),
			item("Hızlı not…", "", a.showQuick),
			item("Geri", "CmdOrCtrl+[", func() { a.step(-1) }),
			item("İleri", "CmdOrCtrl+]", func() { a.step(1) }),
		}},
		{Role: mygo.RoleWindowMenu},
	})
}
