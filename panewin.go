package main

import (
	"encoding/json"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/richtext"
	ryugo "github.com/imhalid/ryu-go"
	"github.com/imhalid/ryu-go/dockwin"
)

// Panes in windows of their own. The panes are a Dock (panes.go); given a host (Ryugo's dockwin), a tab
// dragged out of the window tears off into a window of its own, and a window closed gives its panes back
// to the main one. Where the system does not say where windows are (Wayland), a tab cannot be dragged out
// by the pointer: the command "Bölmeyi ayrı pencereye al" puts the pane in use in a window of its own,
// everywhere. Each window builds its own areas of the one layout; what is drawn over the text (the caret,
// the menus of the text) is drawn in the window that has the pane in use.

// newDockHost makes the host of the panes' windows, the main window being w.
func (a *app) newDockHost(w *mygo.Window) {
	a.dockHost = dockwin.New(w, func() *ryugo.Layout { return a.ps.layout }, func(id string) func(*ui.Context) {
		return func(c *ui.Context) { a.paneWindowView(c, id) }
	})
	a.dockHost.OnChange = func() {
		if a.invalidate != nil {
			a.invalidate()
		}
	}
}

// host is the Dock's host, nil where there is none (the tests, the web).
func (a *app) host() ryugo.DockHost {
	if a.dockHost == nil {
		return nil
	}
	return a.dockHost
}

// paneKinds is the one kind of area the Dock knows: a pane of the app, built in its own state.
func (a *app) paneKinds(th *richtext.Theme) []ryugo.DockKind {
	n := a.paneCount()
	return []ryugo.DockKind{{Kind: "pane", Title: T("Bölme"), Icon: iconPanel, Build: func(c *ui.Context, p *ryugo.Pane) {
		if i := a.ps.index(p.ID); i >= 0 {
			a.withPane(i, func() { a.paneColumn(c, th, i, n, true) })
		}
	}}}
}

// paneWindowView is a window of panes torn off: its areas of the layout, and, when it has the pane in
// use, what is drawn over the text.
func (a *app) paneWindowView(c *ui.Context, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ps.layout == nil || a.ps.layout.Window(id) == nil {
		return
	}
	_, th := a.frameTheme(c)
	a.pageBg = a.bg(c)
	ui.Column(c).Fill().Background(a.pageBg).Children(func() {
		ryugo.Dock(c.Key("panes:"+id), a.ps.layout, ryugo.DockOptions{Window: id, Kinds: a.paneKinds(th), MinSize: 240, Host: a.host()}).Grow(1)
	})
	if a.paneWindow(a.ps.active) == id {
		a.textOverlays(c)
	}
}

// paneWindow is the window of the layout that has pane i: "main" when it is in the main window, or when
// there is no layout.
func (a *app) paneWindow(i int) string {
	l := a.ps.layout
	if l == nil || i < 0 || i >= len(a.ps.ids) {
		return "main"
	}
	id := a.ps.ids[i]
	var in func(n *ryugo.Node) bool
	in = func(n *ryugo.Node) bool {
		if n == nil {
			return false
		}
		for _, p := range n.Panes {
			if p.ID == id {
				return true
			}
		}
		for _, ch := range n.Children {
			if in(ch) {
				return true
			}
		}
		return false
	}
	for _, w := range l.Windows {
		if in(w.Root) {
			return w.ID
		}
	}
	return "main"
}

// activeElsewhere reports whether the pane in use is in a window torn off: the main window does not draw
// what lies over its text.
func (a *app) activeElsewhere() bool { return a.paneWindow(a.ps.active) != "main" }

// tearOffPane puts the pane in use in a window of its own. With one pane, a second one is made first,
// showing the same, so that the main window keeps one.
func (a *app) tearOffPane() {
	if a.paneCount() < 2 { // the new one goes, and is the one in use
		a.newPaneHere()
		if i := a.ps.want; i >= 0 {
			a.ps.want = -1
			a.usePane(i)
		}
	}
	l := a.ps.layout
	if l == nil {
		return
	}
	a.ps.ensureIDs()
	id := a.ps.ids[a.ps.active]
	p := l.FindPane(id)
	if p == nil || a.paneWindow(a.ps.active) != "main" || len(l.PaneIDs()) < 2 {
		return
	}
	pane := *p
	if !l.RemovePane(id) {
		return
	}
	w, h := 640, 560
	if a.winRect.W > 0 {
		w, h = int(a.winRect.W*0.5), int(a.winRect.H*0.8)
	}
	l.Windows = append(l.Windows, &ryugo.DockWindow{ID: "win-" + id, Width: w, Height: h, Root: ryugo.DockArea(&pane)})
	// read back, so that the layout mends what it holds (the new nodes' ids)
	if b, err := json.Marshal(l); err == nil {
		if fixed, err := ryugo.ParseLayout(b); err == nil {
			a.ps.layout = fixed
		}
	}
	if a.dockHost != nil {
		a.dockHost.Sync()
	}
}

// textOverlays are what is drawn over the text of the pane in use: the caret, the bar of a selection,
// the gutter and the grip of a block, the / menu, a date being set, a table's cell being written.
func (a *app) textOverlays(c *ui.Context) {
	a.caretView(c)
	a.selectionBar(c)
	a.gutterView(c)
	a.taskBoxHits(c)
	a.gripView(c)
	a.codeCopyView(c)
	a.slashView(c)
	a.dateEditView(c)
	a.cellEditView(c)
}
