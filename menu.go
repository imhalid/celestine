package main

import "github.com/egoist/mygo"

// The menu bar, where a Mac user looks first: every command with its
// shortcut, made from the app's list of commands (appcmds.go). In the app the
// menu bar has the shortcuts (a.menuBar), so the window doesn't take them
// twice; tests, without one, press them on the window (appKeys).

func (a *app) menu() *mygo.Menu {
	group := func(label, name string, before, after []*mygo.MenuItem) *mygo.MenuItem {
		items := append(append(before, a.menuBarItems(name)...), after...)
		return &mygo.MenuItem{Label: label, Submenu: items}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		group("ryudit", "app", []*mygo.MenuItem{{Role: mygo.RoleAbout}, mygo.Separator()}, []*mygo.MenuItem{
			mygo.Separator(),
			{Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}),
		group(T("Dosya"), "file", nil, []*mygo.MenuItem{mygo.Separator(), {Role: mygo.RoleClose}}),
		{Role: mygo.RoleEditMenu},
		group(T("Görünüm"), "view", nil, []*mygo.MenuItem{mygo.Separator(), {Role: mygo.RoleToggleFullScreen}}),
		group(T("Blok"), "block", nil, nil),
		group(T("Git"), "go", nil, nil),
		{Role: mygo.RoleWindowMenu},
	})
}
