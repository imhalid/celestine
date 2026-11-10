package main

import (
	"bytes"
	"slices"

	"github.com/egoist/mygo/ui"
	ryugo "github.com/imhalid/ryu-go"
)

// What agents changed while the app runs: each page an agent wrote is kept as it was before the agent's first write,
// and the pages an agent made are known. The status bar says how many, and one click puts them all back: the pages
// to what they were, the made ones to the trash. Before each write the page is also kept as a version (in a library
// kept in a file), so one write can be taken back from the page's history.

// agentEdits is the record.
type agentEdits struct {
	pages  []*Page          // in the order the agent came to them
	before map[*Page][]byte // a page's file before the agent's first write
	made   map[*Page]bool   // made by the agent
}

// agentTouch is called before an agent writes page p: its file is kept, the first time, and as a version each time.
func (a *app) agentTouch(p *Page) {
	if p == a.page {
		a.flush()
	}
	if h, ok := historyOf(p); ok {
		h.SnapshotNow(p.File())
	}
	r := &a.agent
	if r.made[p] || r.before[p] != nil {
		return
	}
	data, err := p.FS().ReadFile(p.File())
	if err != nil {
		return
	}
	if r.before == nil {
		r.before = map[*Page][]byte{}
	}
	r.before[p] = data
	r.pages = append(r.pages, p)
}

// agentMade notes that an agent made page p.
func (a *app) agentMade(p *Page) {
	r := &a.agent
	if r.made == nil {
		r.made = map[*Page]bool{}
	}
	r.made[p] = true
	r.pages = append(r.pages, p)
}

// agentChanged is how many pages the agents changed that are still in the library and not as they were.
func (a *app) agentChanged() int {
	n := 0
	for _, p := range a.agent.pages {
		if a.agentDiffers(p) {
			n++
		}
	}
	return n
}

// agentDiffers is whether page p, which an agent wrote or made, is still in the library and not as it was.
func (a *app) agentDiffers(p *Page) bool {
	if !a.inLibrary(p) {
		return false
	}
	if a.agent.made[p] {
		return true
	}
	if p == a.page && a.dirty {
		return true
	}
	now, err := p.FS().ReadFile(p.File())
	return err != nil || !bytes.Equal(now, a.agent.before[p])
}

// inLibrary is whether page p is in a collection of the library, not thrown away.
func (a *app) inLibrary(p *Page) bool {
	for _, c := range a.lib.Collections {
		if slices.Contains(c.All(), p) {
			return true
		}
	}
	return false
}

// undoAgent puts back what the agents changed: each page as it was before them (what it is now kept as a version
// first), and the pages they made to the trash. The record is cleared.
func (a *app) undoAgent() {
	r := a.agent
	a.agent = agentEdits{}
	n := 0
	for _, p := range r.pages {
		if !a.inLibrary(p) {
			continue
		}
		if r.made[p] {
			a.trashPage(p)
			n++
			continue
		}
		if p == a.page {
			a.flush()
		}
		now, _ := p.FS().ReadFile(p.File())
		if bytes.Equal(now, r.before[p]) {
			continue
		}
		if h, ok := historyOf(p); ok {
			h.SnapshotNow(p.File())
		}
		if err := p.FS().WriteFile(p.File(), r.before[p]); err != nil {
			a.fail(err)
			continue
		}
		p.Writes++
		a.takeOutside(p)
		n++
	}
	a.linksStale()
	PagesChanged()
	a.notice = Tf("Ajanın değişiklikleri geri alındı: %d sayfa", n)
}

// agentStatus is the status bar's word of what the agents changed: a click puts it all back, the × keeps it.
func (a *app) agentStatus(c *ui.Context) {
	n := a.agentChanged()
	if n == 0 {
		return
	}
	t := c.Theme()
	ui.Row(c.Key("agent-edits")).Gap(4).AlignItems(ui.Center).Children(func() {
		b := ui.Row(c.Key("agent-undo")).Padding(2, 6).Radius(rad(6)).AlignItems(ui.Center).
			Label(T("Ajanın değişikliklerini geri al")).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		ryugo.Tooltip(c, b, T("Ajanın değişikliklerini geri al"))
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Text(c, Tf("Ajan %d sayfayı değiştirdi", n)).FontSize(11.5).TextColor(t.TextMuted)
		})
		if b.Clicked() {
			a.undoAgent()
		}
		k := ui.Box(c.Key("agent-keep")).Padding(2, 4).Radius(rad(6)).Label(T("Ajanın değişikliklerini tut")).
			Role(ui.RoleButton).Cursor(ui.CursorPointer)
		if k.Hovered() {
			k.Background(t.SurfaceHover)
		}
		k.Children(func() { ui.Text(c, "×").FontSize(12).TextColor(t.TextMuted) })
		if k.Clicked() {
			a.agent = agentEdits{}
		}
	})
}
