package main

import (
	"bytes"
	"slices"

	"github.com/egoist/mygo/ui"
	"github.com/imhalid/celestine/locales"
	ryugo "github.com/imhalid/ryu-go"
)

// What agents changed while the app runs: each page an agent wrote is kept as it was before the agent's first write,
// the pages an agent made are known, and so are the other changes it made through the tools (#180): a page moved,
// thrown away or brought back, a collection made or changed, a property added. The status bar says how many, and one
// click puts them all back: the changes taken back the last first, then the pages to what they were, the made ones
// to the trash. Before each write the page is also kept as a version (in a library kept in a file), so one write can
// be taken back from the page's history. A command of the palette an agent runs (run_command) is the user's own
// action, and is undone as the user would undo it. The record follows the pages by their folders: a folder library
// read again has other pages (reloadLibrary), and a page taken back from the trash is another one.

// agentEdits is the record.
type agentEdits struct {
	pages  []*Page          // in the order the agent came to them
	before map[*Page][]byte // a page's file before the agent's first write
	made   map[*Page]bool   // made by the agent
	dirs   map[*Page]string // where each was when the agent came to it
	ops    []func() error   // the other changes, each as what takes it back
}

// agentTouch is called before an agent writes page p: its file is kept, the first time, and as a version each time.
func (a *app) agentTouch(p *Page) {
	if p == a.pane().page {
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
	r.note(p)
}

// agentMade notes that an agent made page p.
func (a *app) agentMade(p *Page) {
	r := &a.agent
	if r.made == nil {
		r.made = map[*Page]bool{}
	}
	r.made[p] = true
	r.note(p)
}

// note puts page p in the record's order, with where it is.
func (r *agentEdits) note(p *Page) {
	if r.dirs == nil {
		r.dirs = map[*Page]string{}
	}
	r.dirs[p] = p.Dir
	r.pages = append(r.pages, p)
}

// agentDid notes a change an agent made that is not a page's text, by what takes it back: it finds what it needs by
// its folder, as the library may hold other objects by then, and does nothing to what is gone.
func (a *app) agentDid(back func() error) { a.agent.ops = append(a.agent.ops, back) }

// agentRemap follows the record to the pages of a library read again: each page by its folder.
func (a *app) agentRemap() {
	r := &a.agent
	if len(r.pages) == 0 {
		return
	}
	old := *r
	*r = agentEdits{ops: old.ops}
	for _, p := range old.pages {
		q := a.pageAt(old.dirs[p])
		if q == nil {
			continue
		}
		if old.made[p] {
			a.agentMade(q)
			continue
		}
		if r.before == nil {
			r.before = map[*Page][]byte{}
		}
		r.before[q] = old.before[p]
		r.note(q)
	}
}

// agentChanged is how many changes the agents made that are still there: the pages still in the library and not
// as they were, and the other changes.
func (a *app) agentChanged() int {
	n := len(a.agent.ops)
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
	if p == a.pane().page && a.pane().dirty {
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

// collAt is the collection of folder dir, or nil.
func (a *app) collAt(dir string) *Collection {
	for _, c := range a.lib.Collections {
		if c.Dir == dir {
			return c
		}
	}
	return nil
}

// undoAgent puts back what the agents changed: the other changes taken back, the last first, so that what was
// moved or thrown away is where it was; then each page as it was before them (what it is now kept as a version
// first), found where it was, and the pages they made to the trash. The record is cleared.
func (a *app) undoAgent() {
	r := a.agent
	a.agent = agentEdits{}
	n := 0
	for i := len(r.ops) - 1; i >= 0; i-- {
		if err := r.ops[i](); err != nil {
			a.fail(err)
			continue
		}
		n++
	}
	for _, orig := range r.pages {
		p := orig
		if !a.inLibrary(p) { // another object now, where it was (back from the trash), or gone
			if p = a.pageAt(r.dirs[orig]); p == nil {
				continue
			}
		}
		if r.made[orig] {
			a.trashPage(p)
			n++
			continue
		}
		if p == a.pane().page {
			a.flush()
		}
		a.lockDays()
		if a.pageLocked(p) {
			a.sayLocked()
			continue
		}
		now, _ := p.FS().ReadFile(p.File())
		if bytes.Equal(now, r.before[orig]) {
			continue
		}
		if h, ok := historyOf(p); ok {
			h.SnapshotNow(p.File())
		}
		if err := p.FS().WriteFile(p.File(), r.before[orig]); err != nil {
			a.fail(err)
			continue
		}
		p.Writes++
		a.takeOutside(p)
		n++
	}
	a.loadTrash()
	a.linksStale()
	PagesChanged()
	a.say(toastOK, locales.AgentChangesUndone(n))
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
			Label(locales.McpTakeBackAgentsChanges()).Role(ui.RoleButton).Cursor(ui.CursorPointer)
		ryugo.Tooltip(c, b, locales.McpTakeBackAgentsChanges())
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
		b.Children(func() {
			ui.Text(c, locales.AgentChanges(n)).FontSize(11.5).TextColor(t.TextMuted)
		})
		if b.Clicked() {
			a.undoAgent()
		}
		k := ui.Box(c.Key("agent-keep")).Padding(2, 4).Radius(rad(6)).Label(locales.McpKeepAgentsChanges()).
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
